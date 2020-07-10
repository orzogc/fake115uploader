package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/eiannone/keyboard"
	"github.com/valyala/fastjson"
)

// const tokenURL = "https://uplb.115.com/3.0/gettoken.php"

const (
	infoURL       = "https://proapi.115.com/app/uploadinfo"
	sampleInitURL = "https://uplb.115.com/3.0/sampleinitupload.php"
	initURL       = "https://uplb.115.com/3.0/initupload.php?isp=0&appid=0&appversion=%s&format=json&sig=%s"
	resumeURL     = "https://uplb.115.com/3.0/resumeupload.php?isp=0&appid=0&appversion=%s&format=json&sig=%s"
	getinfoURL    = "https://uplb.115.com/3.0/getuploadinfo.php"
	listFileURL   = "https://proapi.115.com/android/2.0/ufile/files?offset=0&user_id=%s&app_ver=%s&show_dir=0&cid=%d"
	appVer        = "23.8.0"
	userAgent     = "Mozilla/5.0 115disk/" + appVer
	endString     = "000000"
	aliUserAgent  = "aliyun-sdk-android/2.9.1"
)

var (
	cmdPath     string // 程序所在文件夹位置
	verbose     *bool  // 是否显示更详细的信息
	cid         uint64 // 115文件夹的cid
	userID      string
	userKey     string
	target      = "U_1_0"
	config      uploadConfig // 设置数据
	success     []string     // 上传成功的文件
	failed      []string     // 上传失败的文件
	quit        = make(chan int)
	multipartCh = make(chan int)
)

// 设置数据
type uploadConfig struct {
	Cookies string `json:"cookies"`
	CID     uint64 `json:"cid"`
}

// 检查错误
func checkErr(err error) {
	if err != nil {
		log.Panicln(err)
	}
}

// 处理输入
func getInput(ctx context.Context) {
	eventCh, err := keyboard.GetKeys(10)
	checkErr(err)
	defer keyboard.Close()
	log.Println("按q键退出程序")

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-eventCh:
			checkErr(event.Err)
			if string(event.Rune) == "q" || string(event.Rune) == "Q" || event.Key == keyboard.KeyCtrlC {
				quit <- 0
				return
			}
		}
	}
}

// 退出处理
func handleQuit() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, os.Kill, syscall.SIGTERM)

	select {
	case <-ch:
	case <-quit:
	}

	log.Println("受到退出信号，正在退出本程序")

	multipartCh <- 0
	<-multipartCh

	exitPrint()
	os.Exit(0)
}

// 程序退出时打印信息
func exitPrint() {
	fmt.Println("上传成功的文件：")
	for _, s := range success {
		fmt.Println(s)
	}
	fmt.Println("上传失败的文件：")
	for _, s := range failed {
		fmt.Println(s)
	}
}

// 获取userID和userKey
func getUserKey() (e error) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("请确定网络是否畅通或者cookies是否设置好，每一次登陆网页端115都要重设一次cookies")
			e = fmt.Errorf("getUserKey() error: %v", err)
		}
	}()

	client := http.Client{}
	req, err := http.NewRequest(http.MethodGet, infoURL, nil)
	checkErr(err)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Cookie", config.Cookies)
	resp, err := client.Do(req)
	checkErr(err)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	checkErr(err)

	var p fastjson.Parser
	v, err := p.ParseBytes(body)
	checkErr(err)
	userID = strconv.Itoa(v.GetInt("user_id"))
	userKey = string(v.GetStringBytes("userkey"))

	if userID == "0" {
		log.Panicln("获取userkey出错，请确定cookies是否设置好")
	}

	if *verbose {
		log.Printf("userID和userKey的值分别是：%s %s", userID, userKey)
	}
	return nil
}

// 读取设置文件
func loadConfig() {
	// 设置文件的文件名
	configFile := "config.json"
	path, err := os.Executable()
	checkErr(err)
	cmdPath = filepath.Dir(path)
	// 设置文件应当在本程序所在文件夹内
	configFile = filepath.Join(cmdPath, configFile)

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		log.Println("设置文件不存在，新建设置文件config.json，请先设置cookies")
		data, err := json.MarshalIndent(config, "", "    ")
		checkErr(err)
		err = ioutil.WriteFile(configFile, data, 0644)
		checkErr(err)
		os.Exit(1)
	} else {
		data, err := ioutil.ReadFile(configFile)
		checkErr(err)
		if json.Valid(data) {
			err = json.Unmarshal(data, &config)
			checkErr(err)
		} else {
			log.Panicln("设置文件config.json的内容不符合json格式，请检查其内容")
			os.Exit(1)
		}
	}

	if config.Cookies == "" {
		log.Println("设置文件config.json里的cookies不能为空字符串")
		os.Exit(1)
	}

	// 去掉last_video_volume
	i := strings.Index(config.Cookies, "last_video_volume=")
	j := strings.Index(config.Cookies, "UID=")
	config.Cookies = config.Cookies[:i] + config.Cookies[j:]

	if *verbose {
		log.Printf("Cookies的值为：%s", config.Cookies)
	}
}

func main() {
	go handleQuit()

	fastUpload := flag.Bool("f", false, "秒传模式上传`文件`")
	upload := flag.Bool("u", false, "先尝试用秒传模式上传`文件`，失败后改用普通模式上传")
	multipartUpload := flag.Bool("m", false, "先尝试用秒传模式上传`文件`，失败后改用断点续传模式上传，可以随时中断下载再重启下载（实验性质，请谨慎使用，注意断点时间不要过长）")
	cidNum := flag.Uint64("c", 0, "上传文件到指定的115文件夹，`cid`为115里的文件夹对应的cid(默认为0，即根目录）")
	verbose = flag.Bool("v", false, "显示更详细的信息（调试用）")
	help := flag.Bool("h", false, "显示帮助信息")

	flag.Parse()
	if flag.NFlag() == 0 {
		log.Println("请输入正确的参数")
		flag.PrintDefaults()
		os.Exit(1)
	}
	if *help {
		flag.PrintDefaults()
		os.Exit(0)
	}

	loadConfig()

	// 优先使用参数指定的cid
	if *cidNum != 0 {
		cid = *cidNum
	} else if config.CID != 0 {
		cid = config.CID
	}
	target = "U_1_" + strconv.FormatUint(cid, 10)

	err := getUserKey()
	checkErr(err)

	defer exitPrint()

	for _, file := range flag.Args() {
		info, err := os.Stat(file)
		checkErr(err)
		if info.IsDir() {
			log.Panicf("%s 是目录，取消上传", file)
			continue
		}

		switch {
		case *fastUpload:
			_, err = fastUploadFile(file)
			if err != nil {
				log.Printf("秒传模式上传 %s 出现错误：%v", file, err)
				failed = append(failed, file)
				continue
			}
			success = append(success, file)
		case *upload:
			token, err := fastUploadFile(file)
			if err != nil {
				log.Printf("秒传模式上传 %s 出现错误：%v", file, err)
				log.Printf("现在开始使用普通模式上传 %s", file)
				err = ossUploadFile(token, file)
				if err != nil {
					log.Printf("普通模式上传 %s 出现错误：%v", file, err)
					failed = append(failed, file)
					continue
				}
			}
			success = append(success, file)
		case *multipartUpload:
			saveFile := filepath.Join(cmdPath, filepath.Base(file)) + ".json"
			log.Println(saveFile)
			info, err := os.Stat(saveFile)
			if os.IsNotExist(err) {
				token, err := fastUploadFile(file)
				if err != nil {
					log.Printf("秒传模式上传 %s 出现错误：%v", file, err)
					log.Println("现在开始使用断点续传模式上传")
					err = multipartUploadFile(token, file, nil)
					if err != nil {
						log.Printf("断点续传模式上传 %s 出现错误：%v", file, err)
						failed = append(failed, file)
						continue
					}
				}
				success = append(success, file)
			} else {
				if info.IsDir() {
					log.Printf("%s 不能是文件夹", saveFile)
					failed = append(failed, file)
					continue
				} else {
					log.Printf("发现文件 %s 的上传曾经中断过，现在开始断点续传", file)
					err = resumeUpload(file)
					if err != nil {
						log.Printf("断点续传模式上传 %s 出现错误：%v", file, err)
						failed = append(failed, file)
						continue
					}
					success = append(success, file)
				}
			}
		default:
			log.Panicln("未知的参数")
		}
	}
}
