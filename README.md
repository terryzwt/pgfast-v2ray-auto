# 介绍
* 自动化[v2ray](https://github.com/v2fly/v2ray-core)客户端。
* 自动根据pgfast的订阅链接生成配置文件，多个服务器形成负载均衡的配置，每三小时自动检查更新.
* 推荐使用chrome浏览器，并使用SwitchyOmega插件进行浏览器代理上网。
* 提供默认http端口：5080， socks5端口：4080，可以通过docker端口映射改写。

# 用法
1. PGFAST_SUBSCRIBE_URL指定pgfast后台的url
2. INTERVAL执行更新时间间隔，单位是分钟。
```bash
git clone git@github.com:terryzwt/pgfast-v2ray-auto.git
cp docker-compose-sample.yml docker-compose.yml
## edit the PGFAST_SUBSCRIBE_URL and INTERVAL
vi pgconfig.toml #里面有两个参数Subscribe_url和Password,可以在https://www.pgfastss.net上找到。需要的是付费用户才行。
docker-compose up -d
```

## 其他
* 构建linux程序
```bash
 CGO_ENABLED=0 GOOS=linux  GOARCH=amd64  go build pgfast.go
 docker-compose build
 docker-compose up -d
```