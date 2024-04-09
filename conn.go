package main

import (
	"errors"
	"net"
	"sync"
	"time"
)

var (
	cacheTTL          int64 = 1000 * 60 * 1 //缓存项存活时间 毫秒
	reserved                = 5             // 连接池 预留连接
	connPoolCache           = map[string]*ConnPool{}
	connPoolCacheLock       = &sync.Mutex{}
)

func GetConnPool(address string) *ConnPool {
	connPoolCacheLock.Lock()
	defer connPoolCacheLock.Unlock()
	pool, find := connPoolCache[address]
	if !find {
		pool = &ConnPool{
			address:      address,
			reserved:     reserved,
			connListLock: &sync.Mutex{},
		}
		// 每分钟检测一次
		pool.ticker = time.NewTicker(time.Minute)
		go pool.cleanup() // 启动清理协程
		go pool.preset()  // 预置
		connPoolCache[address] = pool
	}
	pool.lastUseTime = time.Now().UnixMilli()
	return pool
}

type ConnPool struct {
	address      string
	connList     []*Conn
	connListSize int
	useListSize  int
	connListLock sync.Locker
	lastUseTime  int64 // 最后使用时间
	reserved     int
	ticker       *time.Ticker // 用于定期清理的ticker
	disabled     bool
}

type Conn struct {
	conn        net.Conn
	lastUseTime int64 // 最后使用时间
}

func (this_ *ConnPool) Get() (conn *Conn, err error) {
	this_.connListLock.Lock()
	defer this_.connListLock.Unlock()

	if this_.disabled {
		err = errors.New("conn pool [" + this_.address + "] is disabled")
		return
	}

	if this_.connListSize > 0 {
		conn = this_.connList[0]
		this_.connList = this_.connList[1:]
		this_.connListSize--
	} else {
		conn, err = this_.New()
		if err != nil {
			return
		}
	}
	this_.useListSize++
	conn.lastUseTime = time.Now().UnixMilli()
	this_.lastUseTime = conn.lastUseTime
	go this_.preset() // 预置
	return
}

func (this_ *ConnPool) New() (conn *Conn, err error) {
	netConn, err := net.Dial("tcp", this_.address)
	if err != nil {
		err = errors.New("dial target [" + this_.address + "]  : " + err.Error())
		return
	}
	conn = &Conn{
		conn: netConn,
	}
	return
}

func (this_ *ConnPool) Return(conn *Conn) {
	if conn != nil && conn.conn != nil {
		_ = conn.conn.Close()
	}
	this_.connListLock.Lock()

	this_.useListSize--
	this_.lastUseTime = time.Now().UnixMilli()

	this_.connListLock.Unlock()

	go this_.preset()
	return
}

func (this_ *ConnPool) AddConn(conn *Conn) {
	this_.connListLock.Lock()

	if this_.disabled {
		this_.connListLock.Unlock()
		_ = conn.conn.Close()
		return
	}

	this_.connList = append(this_.connList, conn)
	this_.connListSize++

	this_.connListLock.Unlock()
	return
}
func (this_ *ConnPool) preset() {
	// 已创建大于 预留连接 则直接跳过
	var size = reserved - this_.connListSize
	if size < 1 {
		return
	}
	for i := 0; i < size; i++ {
		if this_.disabled {
			break
		}
		conn, err := this_.New()
		if err != nil {
			continue
		}
		this_.AddConn(conn)
	}
	return
}

func (this_ *ConnPool) cleanup() {
	for {
		select {
		case <-this_.ticker.C:
			var now = time.Now().UnixMilli()
			// 当前时间 - 最后时间时间 表示 空闲的时间  如果 大于最大空闲时间 则需要释放
			if now-this_.lastUseTime > cacheTTL {
				connPoolCacheLock.Lock()
				delete(connPoolCache, this_.address)
				connPoolCacheLock.Unlock()
				this_.disable()
				return
			}
		}
	}
}

func (this_ *ConnPool) disable() {
	this_.disabled = true
	this_.ticker.Stop()
	for _, c := range this_.connList {
		_ = c.conn.Close()
	}
}
