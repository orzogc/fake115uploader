// 不要使用本文件
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/valyala/fastjson"
)

type uploadToken struct {
	Object    string `json:"object"`
	AccessID  string `json:"accessid"`
	Host      string `json:"host"`
	Policy    string `json:"policy"`
	Signature string `json:"signature"`
	Expire    int64  `json:"expire"`
	Callback  string `json:"callback"`
}

// 写入fieldname和相应的值
func writeField(mwriter *multipart.Writer, fieldname string, value string) {
	err := mwriter.WriteField(fieldname, value)
	checkErr(err)
}

// 处理multipart
func readMultiParts(writer *io.PipeWriter, mwriter *multipart.Writer, file string, token uploadToken) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("Recovering from panic in readMultiParts(), the error is: ", err)
		}
	}()
	// 需要关闭writer和mwriter，避免EOF错误
	defer writer.Close()
	defer mwriter.Close()

	filename := filepath.Base(file)
	writeField(mwriter, "name", filename)
	writeField(mwriter, "key", token.Object)
	writeField(mwriter, "policy", token.Policy)
	writeField(mwriter, "OSSAccessKeyId", token.AccessID)
	writeField(mwriter, "success_action_status", "200")
	writeField(mwriter, "callback", token.Callback)
	writeField(mwriter, "signature", token.Signature)
	w, err := mwriter.CreateFormFile("file", filename)
	checkErr(err)

	f, err := os.Open(file)
	checkErr(err)
	defer f.Close()
	_, err = io.Copy(w, f)
	checkErr(err)
}

// 获取上传所需的token
func getUploadToken(file string) (token uploadToken, e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("getUploadToken() error: %w", err)
		}
	}()

	info, err := os.Stat(file)
	checkErr(err)
	form := url.Values{}
	form.Set("userid", userID)
	form.Set("filename", info.Name())
	form.Set("filesize", strconv.FormatInt(info.Size(), 10))
	form.Set("target", target)

	req, err := http.NewRequest(http.MethodPost, sampleInitURL, strings.NewReader(form.Encode()))
	checkErr(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Cookie", config.Cookies)
	resp, err := httpClient.Do(req)
	checkErr(err)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	checkErr(err)

	err = json.Unmarshal(body, &token)
	checkErr(err)

	if *verbose {
		log.Printf("upload toke的值：\n%+v", token)
	}

	return token, nil
}

// 以普通模式上传文件
func uploadFile(file string) (e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("uploadFile() error: %w", err)
		}
	}()

	log.Println("普通模式上传文件：" + file)

	info, err := os.Stat(file)
	checkErr(err)
	if info.Size() > 5*1024*1024*1024 {
		panic(fmt.Errorf("%s 的大小超过5GB，目前上传的单个文件大小不能超过5GB", file))
	}

	token, err := getUploadToken(file)
	checkErr(err)

	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	req, err := http.NewRequest(http.MethodPost, token.Host, reader)
	checkErr(err)

	mwriter := multipart.NewWriter(writer)
	defer mwriter.Close()
	req.Header.Set("Content-Type", mwriter.FormDataContentType())

	go readMultiParts(writer, mwriter, file, token)

	resp, err := httpClient.Do(req)
	checkErr(err)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	checkErr(err)

	if *verbose {
		log.Printf("普通模式上传文件 %s 的response body的内容是：\n%s", file, string(body))
	}

	var p fastjson.Parser
	v, err := p.ParseBytes(body)
	checkErr(err)
	if v.GetBool("state") == true && v.GetInt("code") == 0 {
		log.Printf("普通模式上传 %s 成功", file)
	} else {
		panic(fmt.Errorf("普通模式上传 %s 失败", file))
	}

	return nil
}
