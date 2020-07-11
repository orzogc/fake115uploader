package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/cheggaaa/pb/v3"
	"github.com/valyala/fastjson"
)

// 上传进度存档文件的数据
type saveProgress struct {
	FastToken fastToken
	Chunks    []oss.FileChunk
	Imur      oss.InitiateMultipartUploadResult
	Parts     []oss.UploadPart
}

// 进度监听
type multipartProgressListener struct {
}

// 实现oss.ProgressListener的接口
func (listener *multipartProgressListener) ProgressChanged(event *oss.ProgressEvent) {
	switch event.EventType {
	case oss.TransferStartedEvent:
	case oss.TransferDataEvent:
	case oss.TransferCompletedEvent:
		bar.Add64(event.ConsumedBytes)
	case oss.TransferFailedEvent:
	default:
	}
}

func getBucket(bucketName string) (ot ossToken, bucket *oss.Bucket) {
	ot, err := getOSSToken()
	checkErr(err)
	client, err := oss.New(ot.endpoint, ot.AccessKeyID, ot.AccessKeySecret)
	checkErr(err)
	bucket, err = client.Bucket(bucketName)
	checkErr(err)
	return ot, bucket
}

// 利用oss的接口以multipart的方式上传文件，sp不为nil时恢复上次的上传
func multipartUploadFile(ft fastToken, file string, sp *saveProgress) (e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("multipartUploadFile() error: %v", err)
		}
	}()

	log.Println("断点续传模式上传文件：" + file)

	// 存档文件保存在本程序所在文件夹内
	saveFile := filepath.Join(cmdPath, filepath.Base(file)) + ".json"
	if sp != nil {
		data, err := ioutil.ReadFile(saveFile)
		checkErr(err)
		err = json.Unmarshal(data, sp)
		checkErr(err)
	}

	var chunks []oss.FileChunk
	var imur oss.InitiateMultipartUploadResult
	var parts []oss.UploadPart
	if sp != nil {
		ft = sp.FastToken
		chunks = sp.Chunks
		imur = sp.Imur
		parts = sp.Parts
	}

	ot, bucket := getBucket(ft.Bucket)
	// ossToken一小时后就会失效
	ticker := time.NewTicker(50 * time.Minute)
	defer ticker.Stop()

	cb := base64.StdEncoding.EncodeToString([]byte(ft.Callback.Callback))
	cbVar := base64.StdEncoding.EncodeToString([]byte(ft.Callback.CallbackVar))

	f, err := os.Open(file)
	checkErr(err)
	defer f.Close()
	info, err := f.Stat()
	checkErr(err)

	if sp == nil {
		// 上传的文件大小不能超过115GB
		if info.Size() > 115*1024*1024*1024 {
			return fmt.Errorf("%s 的大小超过115GB，取消上传", file)
		}
		for i := int64(1); i < 10; i++ {
			if info.Size() < i*1024*1024*1024 {
				// 文件大小小于iGB时分为i*1000片
				chunks, err = oss.SplitFileByPartNum(file, int(i*1000))
				checkErr(err)
				break
			}
		}
		if info.Size() > 9*1024*1024*1024 {
			// 文件大小大于9GB时分为10000片
			chunks, err = oss.SplitFileByPartNum(file, 10000)
			checkErr(err)
		}
		// 单个分片大小不能小于100KB
		if chunks[0].Size < 100*1024 {
			chunks, err = oss.SplitFileByPartSize(file, 100*1024)
			checkErr(err)
		}
		imur, err = bucket.InitiateMultipartUpload(ft.Object,
			oss.SetHeader("x-oss-security-token", ot.SecurityToken),
			oss.UserAgentHeader(aliUserAgent),
		)
		checkErr(err)
	}

	bar = pb.Full.Start64(info.Size())
	if sp != nil {
		bar.SetCurrent(int64(len(sp.Parts)) * sp.Chunks[0].Size)
	}
	bar.Set(pb.Bytes, true)
	bar.Set(pb.SIBytesPrefix, true)
	defer bar.Finish()

	fmt.Println("按q键停止下载并退出程序")
	var tempChunks []oss.FileChunk
	if sp != nil {
		tempChunks = chunks[len(sp.Parts):]
	} else {
		tempChunks = chunks
	}
	uploadingPart = true
	defer func() {
		uploadingPart = false
	}()
	for _, chunk := range tempChunks {
		select {
		case <-multipartCh:
			bar.Finish()
			log.Printf("正在保存 %s 的上传进度，存档文件是 %s", file, saveFile)
			sp = &saveProgress{FastToken: ft, Chunks: chunks, Imur: imur, Parts: parts}
			data, err := json.Marshal(*sp)
			checkErr(err)
			err = ioutil.WriteFile(saveFile, data, 0644)
			checkErr(err)
			saved = append(saved, file)
			multipartCh <- 0
			return errors.New("保存进度")
		default:
			var part oss.UploadPart
			// 出现错误就继续尝试，共尝试3次
			for retry := 0; retry < 3; retry++ {
				select {
				case <-ticker.C:
					// 到时重新获取ossToken
					ot, bucket = getBucket(ft.Bucket)
				default:
				}
				f.Seek(chunk.Offset, io.SeekStart)
				part, err = bucket.UploadPart(imur, f, chunk.Size, chunk.Number,
					oss.SetHeader("x-oss-security-token", ot.SecurityToken),
					oss.UserAgentHeader(aliUserAgent),
					oss.Progress(&multipartProgressListener{}),
				)
				if err == nil {
					break
				} else {
					log.Printf("上传 %s 的第%d个分片时出现错误：%v", file, chunk.Number, err)
					if retry != 2 {
						log.Printf("尝试重新上传第%d个分片", chunk.Number)
					}
				}
			}
			if err != nil {
				// 分片上传出现3次错误则保存上传进度
				log.Printf("正在保存 %s 的上传进度，存档文件是 %s", file, saveFile)
				sp = &saveProgress{FastToken: ft, Chunks: chunks, Imur: imur, Parts: parts}
				data, err := json.Marshal(*sp)
				checkErr(err)
				err = ioutil.WriteFile(saveFile, data, 0644)
				checkErr(err)
				saved = append(saved, file)
				return errors.New("保存进度")
			}
			parts = append(parts, part)
		}
	}
	uploadingPart = false
	bar.Finish()

	select {
	case <-ticker.C:
		// 到时重新获取ossToken
		ot, bucket = getBucket(ft.Bucket)
	default:
	}
	var header http.Header
	cmur, err := bucket.CompleteMultipartUpload(imur, parts,
		oss.SetHeader("x-oss-security-token", ot.SecurityToken),
		oss.Callback(cb),
		oss.CallbackVar(cbVar),
		oss.UserAgentHeader(aliUserAgent),
		oss.GetResponseHeader(&header),
	)
	// EOF错误好像是xml的Unmarshal导致的，实际上上传是成功的
	if err != nil && fmt.Sprint(err) != "EOF" {
		log.Panicln(err)
	}
	if *verbose {
		log.Printf("CompleteMultipartUpload的响应头的值是：\n%+v", header)
		log.Printf("cmur的值是：%+v", cmur)
	}

	// 验证上传是否成功
	fileURL := fmt.Sprintf(listFileURL, userID, appVer, config.CID)
	body := getURL(fileURL)
	var p fastjson.Parser
	v, err := p.ParseBytes(body)
	checkErr(err)
	s := string(v.GetArray("data")[0].GetStringBytes("sha1"))
	if s == ft.SHA1 {
		log.Printf("断点续传模式上传 %s 成功", file)
		if sp != nil {
			log.Printf("删除存档文件 %s", saveFile)
			err = os.Remove(saveFile)
			checkErr(err)
		}
	} else {
		log.Panicf("断点续传模式上传 %s 失败", file)
	}

	return nil
}

// 恢复上传文件
func resumeUpload(file string) (e error) {
	sp := new(saveProgress)
	return multipartUploadFile(fastToken{}, file, sp)
}
