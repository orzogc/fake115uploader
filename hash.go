package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// 计算文件指定范围内的sha1值
func hashFileRange(f *os.File, signCheck string) (rangeHash string, e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("hashFileRange() error: %v", err)
		}
	}()

	var start, end int64
	_, err := fmt.Sscanf(signCheck, "%d-%d", &start, &end)
	checkErr(err)
	if start < 0 || end < 0 || end < start {
		return "", fmt.Errorf("sign_check范围错误：%s", signCheck)
	}

	_, err = f.Seek(start, io.SeekStart)
	checkErr(err)
	h := sha1.New()
	_, err = io.CopyN(h, f, end-start+1)
	checkErr(err)

	return strings.ToUpper(hex.EncodeToString(h.Sum(nil))), nil
}

// 计算文件的sha1值
func hashSHA1(f *os.File) (blockHash, totalHash string, e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("hashSHA1() error: %v", err)
		}
	}()

	// 计算文件最前面一个区块的sha1 hash值
	block := make([]byte, 128*1024)
	n, err := f.Read(block)
	checkErr(err)
	data := sha1.Sum(block[:n])
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
