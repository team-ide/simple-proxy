package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/team-ide/go-tool/util"
	"go.uber.org/zap"
	"io"
	"net"
	"sync"
	"time"
)

type Config struct {
	Address string `json:"address"`
}

func NewServer(config *Config) *Server {
	ser := &Server{
		Config: config,
	}
	return ser
}

type Server struct {
	*Config
	server    net.Listener
	isStopped bool
}

func (this_ *Server) Start() (err error) {
	this_.server, err = this_.getServer()
	if err != nil {
		return
	}
	go this_.serverAccept()
	return
}
func (this_ *Server) Stop() {
	this_.isStopped = true
	if this_.server != nil {
		_ = this_.server.Close()
	}
	return
}

func (this_ *Server) getServer() (server net.Listener, err error) {
	if this_.isStopped {
		err = errors.New("server is stopped")
		return
	}
	server, err = net.Listen("tcp", this_.Address)
	if err != nil {
		util.Logger.Error("server listen error", zap.Error(err))
		return
	}
	util.Logger.Info("server listen [" + this_.Address + "] success")
	return
}

func (this_ *Server) serverAccept() {
	defer func() {
		if this_.isStopped {
			return
		}
		if this_.server != nil {
			_ = this_.server.Close()
		}
		for {
			time.Sleep(time.Second * 5)
			if this_.isStopped {
				return
			}
			var err error
			this_.server, err = this_.getServer()
			if err != nil {
				continue
			}
			break
		}
		this_.serverAccept()
	}()
	for {
		client, err := this_.server.Accept()
		if err != nil {
			if this_.isStopped {
				return
			}
			util.Logger.Error("server accept error", zap.Error(err))
			continue
		}
		processor := &Processor{
			Server: this_,
			client: client,
		}
		go processor.process()
	}
}

type Processor struct {
	*Server
	info       string
	client     net.Conn
	targetPool *ConnPool
	target     *Conn
}

func (this_ *Processor) process() {
	this_.info = "process server [" + this_.Address + "] conn [" + this_.client.RemoteAddr().String() + "]"
	defer func() {
		_ = this_.client.Close()
		if this_.target != nil && this_.targetPool != nil {
			this_.targetPool.Return(this_.target)
		}
		// 连接关闭
		util.Logger.Debug(this_.info + " closed")
	}()
	util.Logger.Debug(this_.info + " opened")
	if err := this_.Auth(); err != nil {
		util.Logger.Error(this_.info+" auth error", zap.Error(err))
		return
	}

	wait := &sync.WaitGroup{}
	wait.Add(2)
	go this_.ReadClientToTarget(wait)
	go this_.ReadTargetToClient(wait)
	wait.Wait()
}

func (this_ *Processor) Auth() (err error) {
	buf := make([]byte, 256)

	// VER  本次请求的协议版本号，取固定值 0x05（表示socks 5） 1
	// NMETHODS 客户端支持的认证方式数量，可取值 1~255 1
	// METHODS 可用的认证方式列表 1~255

	// 读取 VER 和 NMETHODS
	n, err := io.ReadFull(this_.client, buf[0:2])
	if err != nil {
		return errors.New("reading header error: " + err.Error())
	}
	// 如果是 http 代理 则 是 `CONNECT www.baidu.com:443 HTTP/1.1` 格式
	// 前 8 个字节为 [67 79 78 78 69 67 84 32]
	if buf[0] == 67 && buf[1] == 79 {
		n, err = io.ReadFull(this_.client, buf[2:8])
		if err != nil {
			return errors.New("reading header error: " + err.Error())
		}
		//fmt.Println("bs:", buf[0:8])
		if string(buf[0:8]) == "CONNECT " {
			var endIndex = 8
			for {
				_, err = io.ReadFull(this_.client, buf[endIndex:endIndex+1])
				if err != nil {
					return errors.New("reading header error: " + err.Error())
				}
				//fmt.Println("read str:", buf[0:endIndex+1])
				//fmt.Println("read str:", string(buf[0:endIndex+1]))
				//fmt.Println("read str:", len(string(buf[0:endIndex+1])))
				if buf[endIndex] == ' ' {
					break
				}
				endIndex++
			}
			toUrl := string(buf[8:endIndex])
			//fmt.Println("toUrl:", toUrl)
			util.Logger.Info(this_.info + " dial target [" + toUrl + "] ")
			this_.targetPool = GetConnPool(toUrl)
			this_.target, err = this_.targetPool.Get()
			if err != nil {
				return errors.New("dial target [" + toUrl + "]  : " + err.Error())
			}

			var bs = make([]byte, byteSize)
			n, err = this_.client.Read(bs)
			if err != nil {
				return
			}
			//fmt.Println("read client:")
			//fmt.Println(string(bs[0:n]))
			n, err = this_.client.Write([]byte(`HTTP/1.1 200 Connection
Content-Length: 0

`))
			if err != nil {
				return errors.New("write rsp err: " + err.Error())
			}
			return
		}

	}

	ver, nMethods := int(buf[0]), int(buf[1])
	if ver != 5 {
		return errors.New("invalid version")
	}

	// 读取 METHODS 列表
	// 选定的认证方式；其中 0x00 表示不需要认证，0x02 是用户名/密码认证
	n, err = io.ReadFull(this_.client, buf[:nMethods])
	if n != nMethods {
		return errors.New("reading methods: " + err.Error())
	}

	//无需认证
	n, err = this_.client.Write([]byte{0x05, 0x00})
	if n != 2 || err != nil {
		return errors.New("write rsp err: " + err.Error())
	}

	err = this_.Connect()
	if err != nil {
		util.Logger.Error(this_.info+" connect error", zap.Error(err))
		return
	}

	return
}

func (this_ *Processor) Connect() error {
	buf := make([]byte, 256)

	n, err := io.ReadFull(this_.client, buf[0:4])
	if err != nil {
		return errors.New("read header error: " + err.Error())
	}
	if n != 4 {
		return errors.New("read header: " + err.Error())
	}
	// VER  0x05，老暗号了  1
	// CMD  连接方式，0x01=CONNECT, 0x02=BIND, 0x03=UDP ASSOCIATE 1
	// RSV 保留字段，现在没用   X'00'
	// ATYP 地址类型，0x01=IPv4，0x03=域名，0x04=IPv6  1
	// DST.ADDR 目标地址                  Variable
	// DST.PORT 目标端口，2字节，网络字节序（network octec order）  2
	ver, cmd, _, atyp := buf[0], buf[1], buf[2], buf[3]
	if ver != 5 || cmd != 1 {
		return errors.New("invalid ver/cmd")
	}
	addr := ""
	switch atyp {
	case 1:
		n, err = io.ReadFull(this_.client, buf[:4])
		if n != 4 {
			return errors.New("invalid IPv4: " + err.Error())
		}
		addr = fmt.Sprintf("%d.%d.%d.%d", buf[0], buf[1], buf[2], buf[3])

	case 3:
		n, err = io.ReadFull(this_.client, buf[:1])
		if n != 1 {
			return errors.New("invalid hostname: " + err.Error())
		}
		addrLen := int(buf[0])

		n, err = io.ReadFull(this_.client, buf[:addrLen])
		if n != addrLen {
			return errors.New("invalid hostname: " + err.Error())
		}
		addr = string(buf[:addrLen])

	case 4:
		return errors.New("IPv6: no supported yet")

	default:
		return errors.New("invalid atyp")
	}

	n, err = io.ReadFull(this_.client, buf[:2])
	if n != 2 {
		return errors.New("read port: " + err.Error())
	}
	port := binary.BigEndian.Uint16(buf[:2])

	targetAddrPort := fmt.Sprintf("%s:%d", addr, port)
	util.Logger.Info(this_.info + " dial target [" + targetAddrPort + "] ")
	this_.targetPool = GetConnPool(targetAddrPort)
	this_.target, err = this_.targetPool.Get()
	if err != nil {
		return errors.New("dial target [" + targetAddrPort + "]  : " + err.Error())
	}
	// 客户端，已经准备好了
	n, err = this_.client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	if err != nil {
		return errors.New("write rsp: " + err.Error())
	}
	return nil
}

var (
	byteSize = 32 * 1024
)

func (this_ *Processor) ReadClientToTarget(wait *sync.WaitGroup) {
	var err error
	defer func() {
		if err != nil {
			util.Logger.Error(this_.info+" read client error", zap.Error(err))
		}
		wait.Done()
	}()
	var buf = make([]byte, byteSize)
	var written int64
	for {
		nr, er := this_.client.Read(buf)
		if nr > 0 {
			//fmt.Println("read client message:")
			//fmt.Println(string(buf[0:nr]))
			nw, ew := this_.target.conn.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errors.New("invalid write result")
				}
			}
			written += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
}

func (this_ *Processor) ReadTargetToClient(wait *sync.WaitGroup) {
	var err error
	defer func() {
		if err != nil {
			util.Logger.Error(this_.info+" read target error", zap.Error(err))
		}
		wait.Done()
	}()
	var buf = make([]byte, byteSize)
	var written int64
	for {
		nr, er := this_.target.conn.Read(buf)
		if nr > 0 {
			//fmt.Println("read target message:")
			//fmt.Println(string(buf[0:nr]))
			nw, ew := this_.client.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errors.New("invalid write result")
				}
			}
			written += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
}
