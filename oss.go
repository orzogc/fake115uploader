package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

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

// 实现 oss.ProgressListener 的接口
func (listener *ossProgressListener) ProgressChanged(event *oss.ProgressEvent) {
	switch event.EventType {
	case oss.TransferStartedEvent:
		bar = pb.New64(event.TotalBytes).SetTemplate(pb.Full).Set(pb.Bytes, true).Start()
	case oss.TransferDataEvent:
		bar.SetCurrent(event.ConsumedBytes)
	case oss.TransferCompletedEvent:
		bar.Finish()
	case oss.TransferFailedEvent:
		bar.Finish()
	default:
	}
}

// 获取网页请求响应的 json
func getURLJSON(url string) (v *fastjson.Value, e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("getURLJSON() error: %v", err)
		}
	}()

	body, err := getURL(url)
	checkErr(err)
	var p fastjson.Parser
	v, err = p.ParseBytes(body)
	checkErr(err)

	return v, nil
}

// 获取 POST 表单请求响应的 json
func postFormJSON(url string, formStr string) (v *fastjson.Value, e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("postFormJSON() error: %v", err)
		}
	}()

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer([]byte(formStr)))
	checkErr(err)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Cookie", config.Cookies)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := doRequest(req)
	checkErr(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	checkErr(err)

	var p fastjson.Parser
	v, err = p.ParseBytes(body)
	checkErr(err)
	return v, nil
}

// 以 GET 请求获取网页内容
func getURL(url string) (body []byte, e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("getURL() error: %v", err)
		}
	}()

	req, err := http.NewRequest(http.MethodGet, url, nil)
	checkErr(err)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Cookie", config.Cookies)
	resp, err := doRequest(req)
	checkErr(err)
	defer resp.Body.Close()
	body, err = io.ReadAll(resp.Body)
	checkErr(err)

	return body, nil
}

// 获取 oss 的 token
func getOSSToken() (token *ossToken, e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("getOSSToken() error: %v", err)
		}
	}()

	token = new(ossToken)
	body, err := getURL(getinfoURL)
	checkErr(err)
	var info uploadInfo
	err = json.Unmarshal(body, &info)
	checkErr(err)
	if *internal {
		i := strings.Index(info.Endpoint, ".aliyuncs.com")
		token.endpoint = info.Endpoint[:i] + "-internal" + info.Endpoint[i:]
	} else {
		token.endpoint = info.Endpoint
	}

	if *verbose {
		log.Printf("info 的值：\n%+v", info)
	}

	body, err = getURL(info.GetTokenURL)
	checkErr(err)
	err = json.Unmarshal(body, &token)
	checkErr(err)

	if *verbose {
		log.Printf("OSS token 的值：\n%+v", token)
	}

	return token, nil
}

// 获取 oss 客户端选项
func getClientOptions() (options []oss.ClientOption) {
	if proxyHost != "" {
		if proxyUser != "" {
			options = append(options, oss.AuthProxy(proxyHost, proxyUser, proxyPassword))
		} else {
			options = append(options, oss.Proxy(proxyHost))
		}
	}

	return options
}

// 利用 oss 的接口上传文件
func ossUploadFile(ft *fastToken, file string) (e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("ossUploadFile() error: %v", err)
		}
	}()

	log.Println("普通模式上传文件：" + file)

	ot, err := getOSSToken()
	checkErr(err)
	client, err := oss.New(ot.endpoint, ot.AccessKeyID, ot.AccessKeySecret, getClientOptions()...)
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

	fmt.Println("按 q 键停止上传并退出程序")
	err = bucket.PutObjectFromFile(ft.Object, file, options...)
	checkErr(err)

	time.Sleep(time.Second)
	// 验证上传是否成功
	fileURL := fmt.Sprintf(listFileURL, config.CID, 20)
	v, err := getURLJSON(fileURL)
	checkErr(err)
	s := string(v.GetStringBytes("data", "0", "sha"))
	if s == ft.SHA1 {
		log.Printf("普通模式上传 %s 成功", file)
		if *removeFile {
			err = remove(file)
			checkErr(err)
		}
	} else {
		panic(fmt.Errorf("普通模式上传 %s 失败", file))
	}

	return nil
}

// 删除文件
func remove(file string) error {
	err := os.Remove(file)
	if err != nil {
		return fmt.Errorf("删除原文件 %s 出现错误：%w", file, err)
	}
	log.Printf("成功删除原文件 %s", file)
	return nil
}
