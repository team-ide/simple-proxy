package main

import (
	"flag"
	"os"
)

var (
	version = "1.0.0"
)

func main() {
	var hasHelp bool
	for _, v := range os.Args {
		if v == "-version" || v == "-v" {
			println(version)
			return
		} else if v == "-help" || v == "-h" {
			hasHelp = true
			break
		}
	}
	address := flag.String("address", ":51210", "监听地址")
	if hasHelp {
		flag.PrintDefaults()
		return
	}
	flag.Parse()
	if *address == "" {
		panic("请配置监听地址（address）")
	}
	config := &Config{
		Address: *address,
	}
	server := NewServer(config)
	err := server.Start()
	if err != nil {
		panic(err)
	}
	<-make(chan int)
}
