package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/cheggaaa/pb/v3"
)

// 上传进度存档文件的数据
type saveProgress struct {
	FastToken *fastToken
	Chunks    []oss.FileChunk
	Imur      oss.InitiateMultipartUploadResult
	Parts     []oss.UploadPart
}

// 进度监听
type multipartProgressListener struct {
}

// 实现 oss.ProgressListener 的接口
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

// 获取 ossToken 和 bucket
func getBucket(bucketName string) (ot *ossToken, bucket *oss.Bucket, e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("getBucket() error: %v", err)
		}
	}()

	ot, err := getOSSToken()
	checkErr(err)
	client, err := oss.New(ot.endpoint, ot.AccessKeyID, ot.AccessKeySecret, getClientOptions()...)
	checkErr(err)
	bucket, err = client.Bucket(bucketName)
	checkErr(err)
	return ot, bucket, nil
}

// 利用 oss 的接口以 multipart 的方式上传文件，sp 不为 nil 时恢复上次的上传
func multipartUploadFile(ft *fastToken, file string, sp *saveProgress) (e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("multipartUploadFile() error: %v", err)
		}
	}()

	log.Println("断点续传模式上传文件：" + file)

	// 存档文件保存在设置文件所在文件夹内
	saveFile := filepath.Join(*saveDir, filepath.Base(file)+".json")
	if sp != nil {
		data, err := os.ReadFile(saveFile)
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

	ot, bucket, err := getBucket(ft.Bucket)
	checkErr(err)
	// ossToken 一小时后就会失效，所以每 50 分钟重新获取一次
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
		// 断点续传模式上传的文件大小不能小于 1KB（1KB 这个大小属于推测，没详细测试过）
		if info.Size() <= 1024 {
			log.Printf("%s 的大小小于1KB，改用普通模式上传", file)
			return ossUploadFile(ft, file)
		}
		// 上传的文件大小不能超过 115GB
		if info.Size() > 115*1024*1024*1024 {
			return fmt.Errorf("%s 的大小超过115GB，取消上传", file)
		}
		// 是否指定分片数量
		if config.PartsNum != 0 {
			chunks, err = oss.SplitFileByPartNum(file, int(config.PartsNum))
			checkErr(err)
		} else {
			for i := int64(1); i < 10; i++ {
				if info.Size() < i*1024*1024*1024 {
					// 文件大小小于 iGB 时分为 i*1000 片
					chunks, err = oss.SplitFileByPartNum(file, int(i*1000))
					checkErr(err)
					break
				}
			}
			if info.Size() > 9*1024*1024*1024 {
				// 文件大小大于 9GB 时分为 10000 片
				chunks, err = oss.SplitFileByPartNum(file, maxParts)
				checkErr(err)
			}
		}
		// 单个分片大小不能小于 100KB
		if chunks[0].Size < 100*1024 {
			chunks, err = oss.SplitFileByPartSize(file, 100*1024)
			checkErr(err)
		}
		imur, err = bucket.InitiateMultipartUpload(ft.Object,
			oss.SetHeader("x-oss-security-token", ot.SecurityToken),
			oss.UserAgentHeader(aliUserAgent),
			oss.Sequential(),
		)
		checkErr(err)
	}

	fmt.Println("按 q 键停止上传并退出程序，断点续传模式会自动保存上传进度")
	bar = pb.New64(info.Size()).SetTemplate(pb.Full).Set(pb.Bytes, true)
	if sp != nil {
		bar.SetCurrent(int64(len(sp.Parts)) * sp.Chunks[0].Size)
	}
	bar.Start()
	defer bar.Finish()

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
			err = os.WriteFile(saveFile, data, 0644)
			checkErr(err)
			result.Saved = append(result.Saved, file)
			multipartCh <- struct{}{}
			return errStopUpload
		default:
			var part oss.UploadPart
			// 出现错误就继续尝试，共尝试 3 次
			for retry := 0; retry < 3; retry++ {
				select {
				case <-ticker.C:
					// 到时重新获取 ossToken
					ot, bucket, err = getBucket(ft.Bucket)
					checkErr(err)
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
				bar.Finish()
				// 分片上传出现 3 次错误则保存上传进度
				log.Printf("正在保存 %s 的上传进度，存档文件是 %s", file, saveFile)
				sp = &saveProgress{FastToken: ft, Chunks: chunks, Imur: imur, Parts: parts}
				data, err := json.Marshal(*sp)
				checkErr(err)
				err = os.WriteFile(saveFile, data, 0644)
				checkErr(err)
				result.Saved = append(result.Saved, file)
				return errStopUpload
			}
			parts = append(parts, part)
		}
	}
	uploadingPart = false
	bar.Finish()

	select {
	case <-ticker.C:
		// 到时重新获取 ossToken
		ot, bucket, err = getBucket(ft.Bucket)
		checkErr(err)
	default:
	}
	var header http.Header
	cmur, err := bucket.CompleteMultipartUpload(imur, parts,
		oss.SetHeader("x-oss-security-token", ot.SecurityToken),
		oss.SetHeader("x-oss-hash-sha1", ft.SHA1),
		oss.Callback(cb),
		oss.CallbackVar(cbVar),
		oss.UserAgentHeader(aliUserAgent),
		oss.GetResponseHeader(&header),
	)
	// EOF 错误是 xml 的 Unmarshal 导致的，响应其实是 json 格式，所以实际上上传是成功的
	if err != nil && !errors.Is(err, io.EOF) {
		// 当文件名含有 &< 这两个字符之一时响应的 xml 解析会出现错误，实际上上传是成功的
		if filename := filepath.Base(file); !strings.ContainsAny(filename, "&<") {
			panic(err)
		}
	}
	if *verbose {
		log.Printf("CompleteMultipartUpload 的响应头的值是：\n%+v", header)
		log.Printf("cmur 的值是：%+v", cmur)
	}

	time.Sleep(time.Second)
	// 验证上传是否成功
	fileURL := fmt.Sprintf(listFileURL, config.CID, 20)
	v, err := getURLJSON(fileURL)
	checkErr(err)
	s := string(v.GetStringBytes("data", "0", "sha"))
	if s == ft.SHA1 {
		log.Printf("断点续传模式上传 %s 成功", file)
		if sp != nil {
			log.Printf("删除存档文件 %s", saveFile)
			err = os.Remove(saveFile)
			checkErr(err)
		}
		if *removeFile {
			f.Close()
			err = remove(file)
			checkErr(err)
		}
	} else {
		panic(fmt.Errorf("断点续传模式上传 %s 失败", file))
	}

	return nil
}

// 恢复上传文件
func resumeUpload(file string) (e error) {
	sp := new(saveProgress)
	return multipartUploadFile(nil, file, sp)
}
