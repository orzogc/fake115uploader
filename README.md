# fake115uploader
模拟115客户端的上传功能，部分实现参考 [Fake115Upload](https://github.com/T3rry7f/Fake115Upload)

### 特点
* 支持秒传模式，需要已经有人上传指定文件到115
* 支持上传超大文件，超过5G的文件需要115 vip会员
* 支持显示上传进度条

### 安装
`go get -u github.com/orzogc/fake115uploader`

### 使用方法
首先要先运行一次 `fake115uploader` 生成设置文件config.json，然后登陆网页版115，按F12后刷新，将115网页请求的Cookie的值全部复制到config.json的cookies的值里

`fake115uploader -f 文件` 秒传模式上传文件

`fake115uploader -u 文件` 先用秒传模式上传文件，失败后改用普通模式上传

可以在config.json或运行时加上参数 `-c cid` 设置cid（默认为0，即根目录），cid为115网盘文件夹的cid，可以登陆115网页版查看网页地址获取cid

### TODO
- [ ] 断点续传
