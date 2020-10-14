# fake115uploader
模拟115网盘客户端的上传功能，部分实现参考 [Fake115Upload](https://github.com/T3rry7f/Fake115Upload)

### 特点
* 支持秒传模式，需要已经有人上传过指定文件到115
* 支持上传超大文件，大小超过5GB的文件需要115 vip会员
* 支持断点续传（适合用于上传超大文件）
* 支持显示上传进度条

### 编译安装
`go get -u github.com/orzogc/fake115uploader`

### 使用方法
首先要先运行一次 `fake115uploader` 生成设置文件config.json（使用 `-d 文件夹` 指定存放设置文件的文件夹，默认为程序所在的文件夹），然后登陆网页版115，按F12后刷新，将115网页请求的Cookie的值全部复制到config.json的cookies的值里（参考[这里](https://github.com/LSD08KM/Fake115Upload_Python3#cookies%E5%9C%A8%E5%93%AA%E9%87%8C)），或者运行时用参数 `-k Cookie` 指定要用的Cookie。

`fake115uploader -f 文件` 秒传模式上传文件，可以指定多个文件且文件必须是最后一个参数，下同。

`fake115uploader -u 文件` 先尝试用秒传模式上传文件，失败后改用普通模式上传。

`fake115uploader -m 文件` 先尝试用秒传模式上传文件，失败后改用断点续传模式上传，可以随时中断上传再重启上传（适合用于上传超大文件，注意暂停上传的时间不要太长）。

`fake115uploader -b 保存文件 文件` 将文件的115 hashlink（115://文件名|文件大小|文件HASH值|块HASH值）追加写入到指定的保存文件。

`fake115uploader -i 文本文件` 从指定的文本文件逐行读取115 hashlink并将其对应文件导入到115中，hashlink可以没有115://前缀。

`fake115uploader -o 保存文件` 从cid指定的115文件夹导出该文件夹内（包括子文件夹）所有文件的115 hashlink到指定的保存文件。

要上传文件到指定的115文件夹，可以在config.json或运行时加上参数 `-c cid` 设置cid（参数设置会覆盖设置文件里的设置，默认为0，即根目录），cid为115文件夹的cid，可以登陆115网页版查看网页地址获取cid。

设置config.json的resultDir或运行时加上参数 `-r 文件夹` 可以将上传结果保存在指定的文件夹内，默认不保存。

运行时加上参数 `-n` 不读取设置文件，这时必须要用 `-k Cookie` 指定115的Cookie。

上传文件时加上参数 `-a` 利用阿里云内网上传文件，需要在阿里云服务器上运行本程序，同时也需要115在服务器的所在地域开通了阿里云OSS，可以在服务器上运行 `curl https://uplb.115.com/3.0/getuploadinfo.php` 查看OSS地域。

上传文件时加上参数 `-e` ，上传成功后自动删除原文件。

运行时加上参数 `-v` 显示更详细的信息（调试用）。
