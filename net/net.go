package net

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"wb3/jotools/chantypes"
	"wb3/jotools/filelog"
	"wb3/jotools/recover_p"
)

type NetManger struct {
	l          *net.TCPListener
	cancelFunc context.Context
	conn       map[int64]*netC
	idd        *int64
	addr       string //链接地址
	cLock      sync.RWMutex
	handle     func([]byte, int64)
	connFailH  func(*netC, chan bool) //连接断开的处理
	sucessH    func()                 //连接成功做的处理
	reConns    *reDatas
}

type netC struct {
	Id     int64
	off    bool
	closec chan bool
	sendc  chan []byte
	reconn chan bool
	conn   net.Conn
}

type nets struct {
	listens []*NetManger
	conns   []*NetManger
}

var netsM *nets

func init() {
	netsM = new(nets)
}

func Stop() {
	for _, netM := range netsM.listens {
		netM.cLock.Lock()
		defer netM.cLock.Unlock()
		for _, v := range netM.conn {
			chantypes.TryWriteChan(v.closec, true, false)

		}
	}

	for _, netM := range netsM.conns {
		netM.cLock.Lock()
		defer netM.cLock.Unlock()
		for _, v := range netM.conn {
			chantypes.TryWriteChan(v.closec, true, false)
		}
	}

}

func StartTcpListen(addr string, h func([]byte, int64), offlineh func(nid int64), successh func(), cancelFunc context.Context) (*NetManger, error) {
	var netM = new(NetManger)
	netM.cancelFunc = cancelFunc
	netM.idd = new(int64)
	netM.conn = make(map[int64]*netC)
	netM.handle = h
	netM.sucessH = successh

	tcpaddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	l, err := net.ListenTCP("tcp", tcpaddr)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	netM.l = l
	netM.connFailH = func(c *netC, _ chan bool) {
		chantypes.TryWriteChan(c.closec, true, false)
		netM.delConnByC(c)
		offlineh(c.Id)
	}
	recover_p.Go(netM.accept)
	netsM.listens = append(netsM.listens, netM)
	filelog.Log("监听成功:" + addr)
	return netM, nil
}

func (n *NetManger) addConn(c *netC) {
	n.cLock.Lock()
	n.conn[c.Id] = c
	n.cLock.Unlock()
}

func (n *NetManger) delConn(id int64) {
	n.cLock.Lock()
	delete(n.conn, id)
	n.cLock.Unlock()
}

func (n *NetManger) delConnByC(c *netC) {
	if c.off {
		return
	}
	n.cLock.Lock()
	delete(n.conn, c.Id)
	n.cLock.Unlock()
	c.off = true
}

func (n *NetManger) getConn(id int64) *netC {
	var res *netC
	n.cLock.RLock()
	res = n.conn[id]
	n.cLock.RUnlock()
	return res
}

func (n *NetManger) newConn(conn net.Conn, ch chan bool) *netC {
	var res = new(netC)
	res.Id = atomic.AddInt64(n.idd, 1)
	res.sendc = make(chan []byte, 1024)
	res.reconn = ch
	res.closec = make(chan bool, 1)
	res.conn = conn
	n.addConn(res)
	return res
}

func (n *NetManger) accept() {
	for {
		c, e := n.l.Accept()
		if e != nil {
			fmt.Println(e)
			return
		}
		filelog.Debug("建立新连接成功," + c.RemoteAddr().String())
		n.sucessH()
		nconn := n.newConn(c, nil)

		recover_p.Go(func() { n.send(nconn, n.cancelFunc) })
		recover_p.Go(func() { n.recv(nconn, n.cancelFunc) })
	}
}

func (n *NetManger) send(c *netC, ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.closec:
			n.delConnByC(c)
			return
		case data := <-c.sendc:
			_, e := c.conn.Write(data)
			if e != nil {
				n.connFailH(c, c.reconn)
				return
			}
		}
	}
}

func (n *NetManger) recv(c *netC, ctx context.Context) {
	var bhead = make([]byte, MaxLength)
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.closec:
			n.delConnByC(c)
			return
		default:

		}

		_, e := io.ReadFull(c.conn, bhead)
		if e != nil {
			n.connFailH(c, c.reconn)
			return
		}

		length := binary.BigEndian.Uint32(bhead) - MaxLength
		if length > 65535 || length < ProtoLen {
			filelog.Error("data too big or little,length:"+strconv.Itoa(int(length)+MaxLength), " ordata:", bhead)
			n.connFailH(c, c.reconn)
			return
		}
		var data = make([]byte, length)
		_, e = io.ReadFull(c.conn, data)
		if e != nil {
			n.connFailH(c, c.reconn)
			return
		}

		n.handle(data, c.Id)
	}
}
