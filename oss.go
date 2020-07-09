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
type ossProgressListener struct {
}

// 实现oss.ProgressListener的接口
func (listener *ossProgressListener) ProgressChanged(event *oss.ProgressEvent) {
	switch event.EventType {
	case oss.TransferStartedEvent:
		bar = pb.Full.Start64(event.TotalBytes)
		bar.Set(pb.Bytes, true)
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

	token, err := getOSSToken()
	checkErr(err)
	client, err := oss.New(token.endpoint, token.AccessKeyID, token.AccessKeySecret)
	checkErr(err)

	bucket, err := client.Bucket(ft.Bucket)
	checkErr(err)

	/*
		info, err := os.Stat(file)
		checkErr(err)
		var callbackVar cbackVar
		// 设置callback
		err = json.Unmarshal([]byte(fToken.Callback.CallbackVar), &callbackVar)
		checkErr(err)
		cb := strings.ReplaceAll(fToken.Callback.Callback, "${bucket}", fToken.Bucket)
		cb = strings.ReplaceAll(cb, "${object}", fToken.Object)
		cb = strings.ReplaceAll(cb, "${size}", strconv.FormatInt(info.Size(), 10))
		cb = strings.ReplaceAll(cb, "${x:pick_code}", callbackVar.PickCode)
		cb = strings.ReplaceAll(cb, "${x:user_id}", callbackVar.UserID)
		cb = strings.ReplaceAll(cb, "${x:behavior_type}", callbackVar.BehaviorType)
		cb = strings.ReplaceAll(cb, "${x:source}", callbackVar.Source)
		cb = strings.ReplaceAll(cb, "${x:target}", callbackVar.Target)

		if *verbose {
			log.Printf("callback的值：\n%s", cb)
		}

		cbBase64 := base64.StdEncoding.EncodeToString([]byte(cb))
	*/

	cb := base64.StdEncoding.EncodeToString([]byte(ft.Callback.Callback))
	cbVar := base64.StdEncoding.EncodeToString([]byte(ft.Callback.CallbackVar))
	options := []oss.Option{
		oss.SetHeader("x-oss-security-token", token.SecurityToken),
		oss.Callback(cb),
		oss.CallbackVar(cbVar),
		oss.UserAgentHeader(aliUserAgent),
		oss.StorageClass(oss.StorageStandard),
		oss.Progress(&ossProgressListener{}),
		//oss.Checkpoint(true, cmdPath+filepath.Base(file)+".cp"),
	}

	err = bucket.PutObjectFromFile(ft.Object, file, options...)
	checkErr(err)

	/*
		f, err := os.Open(file)
		checkErr(err)
		defer f.Close()
		info, err := f.Stat()
		checkErr(err)

		var nextPos int64 = 0
		nextPos, err = bucket.AppendObject(ft.Object, f, nextPos, options...)
		checkErr(err)

		props, err := bucket.GetObjectDetailedMeta(ft.Object, options...)
		checkErr(err)
		nextPos, err = strconv.ParseInt(props.Get(oss.HTTPHeaderOssNextAppendPosition), 10, 64)
		checkErr(err)
		log.Printf("info size: %d nextPos: %d", info.Size(), nextPos)
	*/

	// 验证上传是否成功
	fileURL := fmt.Sprintf(listFileURL, userID, appVer, cid)
	body := getURL(fileURL)
	var p fastjson.Parser
	v, err := p.ParseBytes(body)
	checkErr(err)
	s := string(v.GetArray("data")[0].GetStringBytes("sha1"))
	if s == ft.sha1 {
		log.Printf("普通模式上传 %s 成功", file)
	} else {
		log.Panicf("普通模式上传 %s 失败", file)
	}

	return nil
}
