#!/bin/sh

# 移至脚本目录
cd `dirname $0`

echo `pwd`

ADDRESS=":51210"

if [ -n "$SIMPLE_PROXY_ADDRESS" ]; then
    ADDRESS=$SIMPLE_PROXY_ADDRESS
fi

chmod +x server

./server -address $ADDRESS
