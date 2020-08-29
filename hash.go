package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/valyala/fastjson"
)

// 计算文件的sha1值
func hashSHA1(file string) (blockHash, totalHash string, e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("hashSHA1() error: %w", err)
		}
	}()

	f, err := os.Open(file)
	checkErr(err)
	defer f.Close()

	// 计算文件最前面一个区块的sha1 hash值
	block := make([]byte, 128*1024)
	_, err = f.Read(block)
	checkErr(err)
	data := sha1.Sum(block)
	blockHash = strings.ToUpper(hex.EncodeToString(data[:]))
	_, err = f.Seek(0, io.SeekStart)
	checkErr(err)

	// 计算整个文件的sha1 hash值
	h := sha1.New()
	_, err = io.Copy(h, f)
	checkErr(err)
	totalHash = strings.ToUpper(hex.EncodeToString(h.Sum(nil)))

	return blockHash, totalHash, nil
}

// 生成指定文件的115 hashlink
func hash115Link(file string) (hashLink string, e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("hash115Link() error: %w", err)
		}
	}()

	blockHash, totalHash, err := hashSHA1(file)
	checkErr(err)
	info, err := os.Stat(file)
	checkErr(err)
	hashLink = linkPrefix + info.Name() + "|" + strconv.FormatInt(info.Size(), 10) + "|" + totalHash + "|" + blockHash
	return hashLink, nil
}

// 将指定文件的115 hashlink写入到保存文件内
func write115Link() (e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("write115Link() error: %w", err)
		}
	}()

	f, err := os.OpenFile(*hashFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	checkErr(err)
	defer f.Close()

	for _, file := range flag.Args() {
		log.Printf("开始生成 %s 的115 hashlink", file)
		hashLink, err := hash115Link(file)
		if err != nil {
			log.Printf("生成 %s 的115 hashlink失败，出现错误：%v", file, err)
			continue
		}
		_, err = f.WriteString(hashLink + "\n")
		if err != nil {
			log.Printf("将115 hashlink写入 %s 出现错误：%v", *hashFile, err)
			continue
		}
	}

	return nil
}

// 利用115 hashlink导入文件到115
func upload115Link(hashLink string) (e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("upload115Link() error: %w", err)
		}
	}()

	s := strings.TrimPrefix(hashLink, linkPrefix)
	link := strings.Split(s, "|")

	if len(link) != 4 || len(link[2]) != 40 || len(link[3]) != 40 {
		panicln(fmt.Errorf("%s 不符合115 hashlink的格式", hashLink))
	}
	if _, err := strconv.ParseUint(link[1], 10, 64); err != nil {
		panicln(fmt.Errorf("%s 不符合115 hashlink的格式", hashLink))
	}

	body, err := uploadSHA1(link[0], link[1], link[2], link[3])
	checkErr(err)

	var p fastjson.Parser
	v, err := p.ParseBytes(body)
	checkErr(err)
	if v.GetInt("status") == 2 && v.GetInt("statuscode") == 0 {
		log.Printf("导入115 hashlink成功：%s", hashLink)
	} else {
		panicln(fmt.Errorf("导入115 hashlink失败：%s", hashLink))
	}

	return nil
}

// 将文件里的115 hashlink导入到115
func uploadLinkFile() (e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("uploadLinkFile() error: %w", err)
		}
	}()

	f, err := os.Open(*inputFile)
	checkErr(err)
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		err := upload115Link(scanner.Text())
		if err != nil {
			log.Printf("导入115 hashlink出现错误：%v", err)
			result.Failed = append(result.Failed, scanner.Text())
			continue
		}
		result.Success = append(result.Success, scanner.Text())
	}
	checkErr(scanner.Err())

	return nil
}
