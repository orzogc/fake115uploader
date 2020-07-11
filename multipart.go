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

	ot, err := getOSSToken()
	checkErr(err)
	client, err := oss.New(ot.endpoint, ot.AccessKeyID, ot.AccessKeySecret)
	checkErr(err)
	bucket, err := client.Bucket(ft.Bucket)
	checkErr(err)

	cb := base64.StdEncoding.EncodeToString([]byte(ft.Callback.Callback))
	cbVar := base64.StdEncoding.EncodeToString([]byte(ft.Callback.CallbackVar))

	if sp == nil {
		chunks, err = oss.SplitFileByPartNum(file, 1000)
		checkErr(err)
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

	f, err := os.Open(file)
	checkErr(err)
	defer f.Close()
	info, err := f.Stat()
	checkErr(err)

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
				// 出现3次错误则保存上传进度
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
