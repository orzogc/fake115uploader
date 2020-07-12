package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/valyala/fastjson"
)

type cbackVar struct {
	PickCode     string `json:"x:pick_code"`
	UserID       string `json:"x:user_id"`
	BehaviorType string `json:"x:behavior_type"`
	Source       string `json:"x:source"`
	Target       string `json:"x:target"`
}

type callback struct {
	Callback    string `json:"callback"`
	CallbackVar string `json:"callback_var"`
}

type fastToken struct {
	Request    string   `json:"request"`
	Status     int      `json:"status"`
	StatusCode int      `json:"statuscode"`
	StatusMsg  string   `json:"statusmsg"`
	PickCode   string   `json:"pickcode"`
	Target     string   `json:"target"`
	Version    string   `json:"version"`
	Bucket     string   `json:"bucket"`
	Object     string   `json:"object"`
	Callback   callback `json:"callback"`
	SHA1       string   // 文件的sha1 hash值
}

// 利用文件的sha1 hash值上传文件获取响应
func uploadSHA1(uplbURL, file string, pickCode string) (body []byte, fileSHA1 string, e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("uploadSHA1() error: %w", err)
		}
	}()

	f, err := os.Open(file)
	checkErr(err)
	defer f.Close()
	info, err := f.Stat()
	checkErr(err)

	// 计算文件最前面一个区块的sha1 hash值
	block := make([]byte, 128*1024)
	_, err = f.Read(block)
	checkErr(err)
	data := sha1.Sum(block)
	blockHash := hex.EncodeToString(data[:])
	_, err = f.Seek(0, io.SeekStart)
	checkErr(err)

	// 计算整个文件的sha1 hash值
	h := sha1.New()
	_, err = io.Copy(h, f)
	checkErr(err)
	totalHash := hex.EncodeToString(h.Sum(nil))

	preID := blockHash
	filename := filepath.Base(file)
	fileID := strings.ToUpper(totalHash)
	quickID := fileID
	data = sha1.Sum([]byte(userID + fileID + quickID + pickCode + target + "0"))
	hash := hex.EncodeToString(data[:])
	sigStr := userKey + hash + endString
	data = sha1.Sum([]byte(sigStr))
	sig := strings.ToUpper(hex.EncodeToString(data[:]))
	uploadURL := fmt.Sprintf(uplbURL, appVer, sig)

	if *verbose {
		log.Printf("sig的值是：%s", sig)
	}

	form := url.Values{}
	form.Set("preid", preID)
	form.Set("filename", filename)
	form.Set("quickid", quickID)
	form.Set("user_id", userID)
	form.Set("app_ver", appVer)
	form.Set("filesize", strconv.FormatInt(info.Size(), 10))
	form.Set("userid", userID)
	form.Set("exif", "")
	form.Set("target", target)
	form.Set("fileid", fileID)

	client := http.Client{}
	req, err := http.NewRequest(http.MethodPost, uploadURL, strings.NewReader(form.Encode()))
	checkErr(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Cookie", config.Cookies)
	resp, err := client.Do(req)
	checkErr(err)
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	checkErr(err)

	return body, fileID, nil
}

// 以秒传模式上传文件
func fastUploadFile(file string) (token fastToken, e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("fastUploadFile() error: %w", err)
		}
	}()

	log.Println("秒传模式上传文件：" + file)

	body, fileSHA1, err := uploadSHA1(initURL, file, "")
	checkErr(err)
	token.SHA1 = fileSHA1

	if *verbose {
		log.Printf("秒传模式上传文件 %s 的响应体的内容是：\n%s", file, string(body))
	}

	var p fastjson.Parser
	v, err := p.ParseBytes(body)
	checkErr(err)
	if v.GetInt("status") == 2 && v.GetInt("statuscode") == 0 {
		log.Printf("秒传模式上传 %s 成功", file)
	} else if v.GetInt("status") == 1 && v.GetInt("statuscode") == 0 {
		// 秒传失败的响应包含普通上传模式和断点续传模式的token
		err = json.Unmarshal(body, &token)
		checkErr(err)

		if *verbose {
			log.Printf("秒传模式上传 %s 失败的响应的json内容是：\n%+v", file, token)
		}

		return token, fmt.Errorf("秒传模式上传 %s 失败", file)
	} else {
		log.Panicf("秒传模式上传 %s 失败", file)
	}

	return token, nil
}
