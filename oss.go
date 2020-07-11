package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/cheggaaa/pb/v3"
	"github.com/valyala/fastjson"
)

var bar *pb.ProgressBar // 上传进度条

type uploadInfo struct {
	Endpoint    string `json:"endpoint"`
	GetTokenURL string `json:"gettokenurl"`
}

type ossToken struct {
	StatusCode      string
	AccessKeySecret string
	SecurityToken   string
	Expiration      string
	AccessKeyID     string `json:"AccessKeyId"`
	endpoint        string
}

// 进度监听
type ossProgressListener struct{}

// 实现oss.ProgressListener的接口
func (listener *ossProgressListener) ProgressChanged(event *oss.ProgressEvent) {
	switch event.EventType {
	case oss.TransferStartedEvent:
		bar = pb.Full.Start64(event.TotalBytes)
		bar.Set(pb.Bytes, true)
		bar.Set(pb.SIBytesPrefix, true)
	case oss.TransferDataEvent:
		bar.SetCurrent(event.ConsumedBytes)
	case oss.TransferCompletedEvent:
		bar.Finish()
	case oss.TransferFailedEvent:
		bar.Finish()
	default:
	}
}

// 以GET请求获取网页内容
func getURL(url string) (body []byte) {
	client := http.Client{}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	checkErr(err)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Cookie", config.Cookies)
	resp, err := client.Do(req)
	checkErr(err)
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	checkErr(err)

	return body
}

// 获取oss的token
func getOSSToken() (token ossToken, e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("getOSSToken() error: %v", err)
		}
	}()

	body := getURL(getinfoURL)
	var info uploadInfo
	err := json.Unmarshal(body, &info)
	checkErr(err)
	token.endpoint = info.Endpoint

	if *verbose {
		log.Printf("info的值：\n%+v", info)
	}

	body = getURL(info.GetTokenURL)
	err = json.Unmarshal(body, &token)
	checkErr(err)

	if *verbose {
		log.Printf("OSS token的值：\n%+v", token)
	}

	return token, nil
}

// 利用oss的接口上传文件
func ossUploadFile(ft fastToken, file string) (e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("ossUploadFile() error: %v", err)
		}
	}()

	log.Println("普通模式上传文件：" + file)

	ot, err := getOSSToken()
	checkErr(err)
	client, err := oss.New(ot.endpoint, ot.AccessKeyID, ot.AccessKeySecret)
	checkErr(err)
	bucket, err := client.Bucket(ft.Bucket)
	checkErr(err)

	cb := base64.StdEncoding.EncodeToString([]byte(ft.Callback.Callback))
	cbVar := base64.StdEncoding.EncodeToString([]byte(ft.Callback.CallbackVar))
	options := []oss.Option{
		oss.SetHeader("x-oss-security-token", ot.SecurityToken),
		oss.Callback(cb),
		oss.CallbackVar(cbVar),
		oss.UserAgentHeader(aliUserAgent),
		oss.Progress(&ossProgressListener{}),
	}

	fmt.Println("按q键停止下载并退出程序")
	err = bucket.PutObjectFromFile(ft.Object, file, options...)
	checkErr(err)

	// 验证上传是否成功
	fileURL := fmt.Sprintf(listFileURL, userID, appVer, config.CID)
	body := getURL(fileURL)
	var p fastjson.Parser
	v, err := p.ParseBytes(body)
	checkErr(err)
	s := string(v.GetArray("data")[0].GetStringBytes("sha1"))
	if s == ft.SHA1 {
		log.Printf("普通模式上传 %s 成功", file)
	} else {
		log.Panicf("普通模式上传 %s 失败", file)
	}

	return nil
}
