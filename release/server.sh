#!/bin/sh

# 移至脚本目录
cd `dirname $0`

echo `pwd`

mkdir -p logs

ADDRESS=":51210"

if [ -n "$SIMPLE_PROXY_ADDRESS" ]; then
    ADDRESS=$SIMPLE_PROXY_ADDRESS
fi

# 添加启动命令
function start(){
    echo "start..."

    chmod +x server
    echo " ldd server"
    ldd server
    nohup ./server simple-proxy-server -address $ADDRESS > logs/start.log 2>&1 &

    echo "start successful"
    return 0
}

# 添加停止命令
function stop(){
    echo "stop..."

    ps aux |grep simple-proxy-server |grep -v grep |awk '{print "kill -15 " $2}'|sh

    echo "stop successful"
    return 0
}

function version(){
    echo "version..."
    chmod +x server
    ./server -v
    return 0
}

case $1 in
"start")
    start
    ;;
"stop")
    stop
    ;;
"restart")
    stop && start
    ;;
"v")
    version
    ;;
"version")
    version
    ;;
*)
    echo "请输入: start, stop, restart, v"
    ;;
esac
