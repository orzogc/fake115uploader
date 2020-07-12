package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/valyala/fastjson"
)

// const tokenURL = "https://uplb.115.com/3.0/gettoken.php"
// const resumeURL = "https://uplb.115.com/3.0/resumeupload.php?isp=0&appid=0&appversion=%s&format=json&sig=%s"

const (
	infoURL       = "https://proapi.115.com/app/uploadinfo"
	sampleInitURL = "https://uplb.115.com/3.0/sampleinitupload.php"
	initURL       = "https://uplb.115.com/3.0/initupload.php?isp=0&appid=0&appversion=%s&format=json&sig=%s"
	getinfoURL    = "https://uplb.115.com/3.0/getuploadinfo.php"
	listFileURL   = "https://proapi.115.com/android/2.0/ufile/files?offset=0&user_id=%s&app_ver=%s&show_dir=0&cid=%d"
	appVer        = "23.8.0"
	userAgent     = "Mozilla/5.0 115disk/" + appVer
	endString     = "000000"
	aliUserAgent  = "aliyun-sdk-android/2.9.1"
)

var (
	fastUpload      *bool
	upload          *bool
	multipartUpload *bool
	configDir       *string
	resultDir       *string
	verbose         *bool
	userID          string
	userKey         string
	target          = "U_1_0"
	config          uploadConfig // 设置数据
	result          resultData   // 上传结果
	uploadingPart   bool
	errStopUpload   = errors.New("暂停上传")
	quit            = make(chan int)
	multipartCh     = make(chan int)
)

// 设置数据
type uploadConfig struct {
	Cookies string `json:"cookies"`
	CID     uint64 `json:"cid"`
}

// 上传结果数据
type resultData struct {
	Success []string `json:"success"` // 上传成功的文件
	Failed  []string `json:"failed"`  // 上传失败的文件
	Saved   []string `json:"saved"`   // 保存上传进度的文件
}

// 检查错误
func checkErr(err error) {
	if err != nil {
		log.Panicln(err)
	}
}

// 获取时间
func getTime() string {
	t := time.Now()
	timeStr := fmt.Sprintf("%d-%02d-%02d %02d-%02d-%02d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
	return timeStr
}

// 处理输入
func getInput(ctx context.Context) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("getInput() error: %v", err)
		}
	}()

	eventCh, err := keyboard.GetKeys(10)
	checkErr(err)
	defer keyboard.Close()

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

	log.Println("收到退出信号，正在退出本程序，请等待")

	if uploadingPart {
		multipartCh <- 0
		<-multipartCh
	}

	keyboard.Close()
	exitPrint()
	os.Exit(0)
}

// 程序退出时打印信息
func exitPrint() {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("exitPrint() error: %v", err)
			// 不保存上传结果
			*resultDir = ""
			exitPrint()
		}
	}()

	if *resultDir != "" {
		resultFile := filepath.Join(*resultDir, getTime()+" result.json")
		log.Printf("上传结果保存在 %s", resultFile)
		data, err := json.MarshalIndent(result, "", "    ")
		checkErr(err)
		err = ioutil.WriteFile(resultFile, data, 0644)
		checkErr(err)
	}

	fmt.Println("上传成功的文件：")
	for _, s := range result.Success {
		fmt.Println(s)
	}
	fmt.Println("上传失败的文件：")
	for _, s := range result.Failed {
		fmt.Println(s)
	}
	fmt.Println("保存上传进度的文件：")
	for _, s := range result.Saved {
		fmt.Println(s)
	}
}

// 获取userID和userKey
func getUserKey() (e error) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("请确定网络是否畅通或者cookies是否设置好，每一次登陆网页端115都要重设一次cookies")
			e = fmt.Errorf("getUserKey() error: %w", err)
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
func loadConfig() (e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("loadConfig() error: %w", err)
		}
	}()

	// 设置文件的文件名
	configFile := "config.json"
	// 设置文件应当在本程序所在文件夹内
	configFile = filepath.Join(*configDir, configFile)

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

	// 去掉last_video_volume
	//i := strings.Index(config.Cookies, "last_video_volume=")
	//j := strings.Index(config.Cookies, "UID=")
	//config.Cookies = config.Cookies[:i] + config.Cookies[j:]

	return nil
}

// 程序初始化
func initialize() (e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("initialize() error: %w", err)
		}
	}()

	fastUpload = flag.Bool("f", false, "秒传模式上传`文件`")
	upload = flag.Bool("u", false, "先尝试用秒传模式上传`文件`，失败后改用普通模式上传")
	multipartUpload = flag.Bool("m", false, "先尝试用秒传模式上传`文件`，失败后改用断点续传模式上传，可以随时中断上传再重启上传（适合用于上传超大文件，注意暂停上传的时间不要太长）")
	configDir = flag.String("d", "", "指定存放设置文件和断点续传存档文件的`文件夹`")
	cookies := flag.String("k", "", "使用指定的115的`Cookie`")
	cid := flag.Uint64("c", 0, "上传文件到指定的115文件夹，`cid`为115里的文件夹对应的cid(默认为0，即根目录）")
	noConfig := flag.Bool("n", false, "不读取设置文件config.json，需要和 -k 配合使用")
	resultDir = flag.String("r", "", "将上传结果保存在指定`文件夹`")
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
	if (*fastUpload && *upload) || (*fastUpload && *multipartUpload) || (*upload && *multipartUpload) {
		log.Println("-f、-u和-m这三个参数只能同时用其中一个")
		os.Exit(1)
	}

	if *configDir == "" {
		path, err := os.Executable()
		checkErr(err)
		*configDir = filepath.Dir(path)
	}

	if !*noConfig {
		err := loadConfig()
		checkErr(err)
	}

	// 优先使用参数指定的Cookie
	if *cookies != "" {
		config.Cookies = *cookies
	}
	if config.Cookies == "" {
		log.Println("设置文件config.json里的cookies不能为空字符串，或者用-k指定115的Cookie")
		os.Exit(1)
	}
	if *verbose {
		log.Printf("Cookies的值为：%s", config.Cookies)
	}

	// 优先使用参数指定的cid
	if *cid != 0 {
		config.CID = *cid
	}
	target = "U_1_" + strconv.FormatUint(config.CID, 10)

	err := getUserKey()
	checkErr(err)

	return nil
}

func main() {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("main() error: %v", err)
		}
	}()

	go handleQuit()

	err := initialize()
	checkErr(err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go getInput(ctx)
	defer keyboard.Close()

	defer exitPrint()

	for _, file := range flag.Args() {
		// 等待一秒
		time.Sleep(time.Second)

		info, err := os.Stat(file)
		checkErr(err)
		if info.IsDir() {
			log.Panicf("%s 是目录，取消上传", file)
			continue
		}

		switch {
		case *fastUpload:
			_, err := fastUploadFile(file)
			if err != nil {
				log.Printf("秒传模式上传 %s 出现错误：%v", file, err)
				result.Failed = append(result.Failed, file)
				continue
			}
			result.Success = append(result.Success, file)
		case *upload:
			token, err := fastUploadFile(file)
			if err != nil {
				log.Printf("秒传模式上传 %s 出现错误：%v", file, err)
				log.Printf("现在开始使用普通模式上传 %s", file)
				err := ossUploadFile(token, file)
				if err != nil {
					log.Printf("普通模式上传 %s 出现错误：%v", file, err)
					result.Failed = append(result.Failed, file)
					continue
				}
			}
			result.Success = append(result.Success, file)
		case *multipartUpload:
			// 存档文件保存在本程序所在文件夹内
			saveFile := filepath.Join(*configDir, filepath.Base(file)) + ".json"
			info, err := os.Stat(saveFile)
			if os.IsNotExist(err) {
				token, err := fastUploadFile(file)
				if err != nil {
					log.Printf("秒传模式上传 %s 出现错误：%v", file, err)
					log.Println("现在开始使用断点续传模式上传")
					err := multipartUploadFile(token, file, nil)
					if err != nil {
						if errors.Is(err, errStopUpload) {
							continue
						}
						log.Printf("断点续传模式上传 %s 出现错误：%v", file, err)
						result.Failed = append(result.Failed, file)
						continue
					}
				}
				result.Success = append(result.Success, file)
			} else {
				if info.IsDir() {
					log.Printf("%s 不能是文件夹", saveFile)
					result.Failed = append(result.Failed, file)
					continue
				}
				log.Printf("发现文件 %s 的上传曾经中断过，现在开始断点续传", file)
				err := resumeUpload(file)
				if err != nil {
					if errors.Is(err, errStopUpload) {
						continue
					}
					log.Printf("断点续传模式上传 %s 出现错误：%v", file, err)
					result.Failed = append(result.Failed, file)
					continue
				}
				result.Success = append(result.Success, file)
			}
		}
	}
	// 等待一秒
	time.Sleep(time.Second)
}
