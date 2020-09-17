package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/valyala/fastjson"
)

// 根据pickcode获取blockhash
func getBlockHash(pickCode string) (blockHash string, e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("getBlockHash() error: %w", err)
		}
	}()

	dlURL := fmt.Sprintf(downloadURL, pickCode)
	client := http.Client{}
	req, err := http.NewRequest(http.MethodGet, dlURL, nil)
	checkErr(err)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Cookie", config.Cookies)
	resp, err := client.Do(req)
	checkErr(err)
	defer resp.Body.Close()

	setCookie := resp.Header["Set-Cookie"]
	cookies := make([]string, len(setCookie))
	for i, cookie := range setCookie {
		if *verbose {
			log.Printf("响应要求设置的Cookie是：%v", cookie)
		}

		cookies[i] = strings.Split(cookie, ";")[0]
	}

	body, err := ioutil.ReadAll(resp.Body)
	checkErr(err)
	var p fastjson.Parser
	v, err := p.ParseBytes(body)
	checkErr(err)

	fileURL := string(v.GetStringBytes("file_url"))
	if *verbose {
		log.Printf("下载地址是：%s", fileURL)
	}

	req, err = http.NewRequest(http.MethodGet, fileURL, nil)
	checkErr(err)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Cookie", strings.Join(cookies, ";"))
	req.Header.Set("Range", "bytes=0-131071")
	resp, err = client.Do(req)
	checkErr(err)
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	checkErr(err)
	data := sha1.Sum(body)
	blockHash = hex.EncodeToString(data[:])

	return blockHash, nil
}

// 导出115 hashlink到指定文本文件
func exportHashLink() (e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("exportHashLink() error: %w", err)
		}
	}()

	fileURL := fmt.Sprintf(listFileURL, 20, userID, appVer, config.CID)
	v, err := getURLJSON(fileURL)
	checkErr(err)
	path := v.GetArray("path")
	if string(path[len(path)-1].GetStringBytes("cid")) != strconv.FormatUint(config.CID, 10) {
		panic(fmt.Errorf("cid %d 不正确", config.CID))
	}
	count := v.GetUint("count")

	fileURL = fmt.Sprintf(listFileURL, count, userID, appVer, config.CID)
	v, err = getURLJSON(fileURL)
	checkErr(err)
	data := v.GetArray("data")
	if len(data) == 0 {
		panic(fmt.Errorf("无法获取cid为 %d 的文件夹下的文件列表", config.CID))
	}

	f, err := os.OpenFile(*outputFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	checkErr(err)
	defer f.Close()

	for _, file := range data {
		filename := string(file.GetStringBytes("fn"))
		fileSize := string(file.GetStringBytes("fs"))
		totalHash := string(file.GetStringBytes("sha1"))
		pickCode := string(file.GetStringBytes("pc"))

		log.Printf("正在获取 %s 的115 hashlink", filename)

		blockHash, err := getBlockHash(pickCode)
		if err != nil {
			log.Printf("无法获取 %s 的blockhash，出现错误：%v", filename, err)
			continue
		}

		hashLink := linkPrefix + filename + "|" + fileSize + "|" + strings.ToUpper(totalHash) + "|" + strings.ToUpper(blockHash)
		_, err = f.WriteString(hashLink + "\n")
		if err != nil {
			log.Printf("将115 hashlink写入 %s 出现错误：%v", *outputFile, err)
			continue
		}
	}

	return nil
}
