# simple-proxy

简单网络代理


## 使用方式

* 下载 `simple-proxy-amd64-xxx.zip`、`simple-proxy-arm64-xxx.zip` 或 docker 运行
* 环境变量
  * SIMPLE_PROXY_ADDRESS：监听地址，用于代理连接，默认 `:51210`
* 运行
```shell

# 解压 `simple-proxy-amd64-xxx.zip`或`simple-proxy-arm64-xxx.zip`包

unzip simple-proxy-amd64-xxx.zip
cd simple-proxy

# 启动
./server.sh start

# 停止
./server.sh start
```
* Docker 运行

```shell

# 最新版本 至 https://hub.docker.com/repository/docker/teamide/simple-proxy/tags?page=1&ordering=last_updated 查看

# amd64 环境
docker run -itd --name simple-proxy-51210 -p 51210:51210 -e SIMPLE_PROXY_ADDRESS=:51210 teamide/simple-proxy:latest

# arm64 环境
docker run -itd --name simple-proxy-51210 -p 51210:51210 -e SIMPLE_PROXY_ADDRESS=:51210 teamide/simple-proxy-arm64:latest


```
