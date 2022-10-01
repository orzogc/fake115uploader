package main

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/valyala/fastjson"
)

/*
type cbackVar struct {
	PickCode     string `json:"x:pick_code"`
	UserID       string `json:"x:user_id"`
	BehaviorType string `json:"x:behavior_type"`
	Source       string `json:"x:source"`
	Target       string `json:"x:target"`
}
*/

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

const md5Salt = "Qclm8MGWUv59TnrR0XPg"

// 上传SHA1的值到115
func uploadSHA1(filename, fileSize, totalHash, blockHash string, targetCID uint64) (body []byte, e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("uploadSHA1() error: %v", err)
		}
	}()

	preID := strings.ToUpper(blockHash)
	fileID := strings.ToUpper(totalHash)
	quickID := fileID
	target := targetPrefix + strconv.FormatUint(targetCID, 10)
	data := sha1.Sum([]byte(userID + fileID + quickID + target + "0"))
	hash := hex.EncodeToString(data[:])
	sigStr := userKey + hash + endString
	data = sha1.Sum([]byte(sigStr))
	sig := strings.ToUpper(hex.EncodeToString(data[:]))

	t := time.Now().Unix()

	userIdMd5 := md5.Sum([]byte(userID))
	tokenMd5 := md5.Sum([]byte(md5Salt + fileID + fileSize + preID + userID + strconv.FormatInt(t, 10) + hex.EncodeToString(userIdMd5[:]) + appVer))
	token := hex.EncodeToString(tokenMd5[:])

	encodedToken, err := ecdhCipher.EncodeToken(t)
	checkErr(err)

	uploadURL := fmt.Sprintf(initURL, t, token, appVer, sig, encodedToken)

	if *verbose {
		log.Printf("initupload的链接是：%s", uploadURL)
		log.Printf("sig的值是：%s", sig)
		log.Printf("k_ec的值是：%s", encodedToken)
	}

	form := url.Values{}
	form.Set("preid", preID)
	form.Set("filename", filename)
	form.Set("quickid", quickID)
	form.Set("user_id", userID)
	form.Set("app_ver", appVer)
	form.Set("filesize", fileSize)
	form.Set("userid", userID)
	form.Set("exif", "")
	form.Set("target", target)
	form.Set("fileid", fileID)

	encrypted, err := ecdhCipher.Encrypt([]byte(form.Encode()))
	checkErr(err)

	req, err := http.NewRequest(http.MethodPost, uploadURL, bytes.NewReader(encrypted))
	checkErr(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Cookie", config.Cookies)
	resp, err := doRequest(req)
	checkErr(err)
	defer resp.Body.Close()
	body, err = io.ReadAll(resp.Body)
	checkErr(err)
	decrypted, err := ecdhCipher.Decrypt(body)
	if err != nil {
		if *verbose {
			log.Printf("解密响应体出现错误：%v", err)
		}

		return body, nil
	}

	return decrypted, nil
}

// 利用文件的sha1 hash值上传文件获取响应
func (file *fileInfo) uploadFileSHA1() (body []byte, fileSHA1 string, e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("uploadFileSHA1() error: %v", err)
		}
	}()

	blockHash, totalHash, err := hashSHA1(file.Path)
	checkErr(err)

	info, err := os.Stat(file.Path)
	checkErr(err)

	body, err = uploadSHA1(info.Name(), strconv.FormatInt(info.Size(), 10), totalHash, blockHash, file.ParentID)
	checkErr(err)

	return body, totalHash, nil
}

// 以秒传模式上传文件
func (file *fileInfo) fastUploadFile() (token *fastToken, e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("fastUploadFile() error: %v", err)
		}
	}()

	token = new(fastToken)
	log.Println("秒传模式上传文件：" + file.Path)

	body, fileSHA1, err := file.uploadFileSHA1()
	checkErr(err)
	token.SHA1 = fileSHA1

	if *verbose {
		log.Printf("秒传模式上传文件 %s 的响应体的内容是：\n%s", file.Path, string(body))
	}

	var p fastjson.Parser
	v, err := p.ParseBytes(body)
	checkErr(err)
	if v.GetInt("status") == 2 && v.Exists("statuscode") && v.GetInt("statuscode") == 0 {
		log.Printf("秒传模式上传 %s 成功", file.Path)
		if *removeFile {
			err = remove(file.Path)
			checkErr(err)
		}
	} else if v.GetInt("status") == 1 && v.Exists("statuscode") && v.GetInt("statuscode") == 0 {
		// 秒传失败的响应包含普通上传模式和断点续传模式的token
		err = json.Unmarshal(body, &token)
		checkErr(err)

		if *verbose {
			log.Printf("秒传模式上传 %s 失败返回的内容是：\n%+v", file.Path, token)
		}

		return token, fmt.Errorf("秒传模式上传 %s 失败", file.Path)
	} else {
		panic(fmt.Errorf("秒传模式上传 %s 失败", file.Path))
	}

	return token, nil
}
