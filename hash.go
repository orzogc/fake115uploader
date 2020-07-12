package main

import (
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	blockHash = hex.EncodeToString(data[:])
	_, err = f.Seek(0, io.SeekStart)
	checkErr(err)

	// 计算整个文件的sha1 hash值
	h := sha1.New()
	_, err = io.Copy(h, f)
	checkErr(err)
	totalHash = hex.EncodeToString(h.Sum(nil))

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
	hashLink = "115://" + filepath.Base(file) + "|" + strconv.FormatInt(info.Size(), 10) + "|" + strings.ToUpper(totalHash) + "|" + strings.ToUpper(blockHash)
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
		hashLink, err := hash115Link(file)
		checkErr(err)
		f.WriteString(hashLink + "\n")
	}

	return nil
}
