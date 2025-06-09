package net

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"io"
	"net"
	"strconv"
	"sync"
	"ysj/jotools/chantypes"
	"ysj/jotools/filelog"
	"ysj/jotools/recover_p"
)

const (
	HEART   = 65535 //心跳消息
	SYNCMSG = 29999 //同步消息

	P_TCP = 1
	P_WS  = 2
)

type NetInter interface {
	SendTimeById(msg int64, id int64, protoid uint16)
	SendById(msg interface{}, id int64, protoid uint16)
	SendByIds(msg proto.Message, ids []int64, protoid uint16)
	SendMsg(msg interface{}, protoid uint16)
	SendAllConn(msg interface{}, protoid uint16)
	Close(nid int64)
	GetConnByCid(cid int64) *NetC
	String() string
}

type NetManger struct {
	l          *net.TCPListener
	ws         bool //是否是websocket
	cancelFunc context.Context
	conn       map[int64]*NetC
	idd        *int64
	addr       string //链接地址
	cLock      sync.RWMutex
	handle     func([]byte, int64)
	connFailH  func(*NetC)              //连接断开的处理
	sucessH    func(...interface{}) int //连接成功做的处理.
	checkHeart bool                     //检测心跳
}

type NetC struct {
	Id        int64       //唯一id
	off       bool        //连接关闭
	CloseC    chan bool   //关闭通道
	SendC     chan []byte //发送通道
	conn      net.Conn    //链接
	connws    *websocket.Conn
	lastHeart bool                             //心跳，期间内任何收发消息都会将其置为 true
	parseH    func(interface{}, uint16) []byte //序列化方法
	readData  func() ([]byte, uint16, error)   //读取方法
	sendData  func([]byte) error               //发送方法
	close     func()                           //关闭
}

type nets struct {
	wsNet   []*NetManger
	listens []*NetManger
	conns   []*ConnManger
}

var netsM *nets

func init() {
	netsM = new(nets)
}

func Stop() {
	for _, netM := range netsM.listens {
		netM.cLock.Lock()
		for _, v := range netM.conn {
			chantypes.TryWriteChan(v.CloseC, true, false)
		}
		netM.cLock.Unlock()
	}

	for _, v := range netsM.conns {
		chantypes.TryWriteChan(v.CloseC, true, false)
	}

	for _, netM := range netsM.wsNet {
		netM.cLock.Lock()
		for _, v := range netM.conn {
			chantypes.TryWriteChan(v.CloseC, true, false)
		}
		netM.cLock.Unlock()
	}

}

func StartTcpListen(addr string, h func([]byte, int64), offlineh func(nid int64), successh func(...interface{}) int, cancelFunc context.Context, checkHeart bool) (*NetManger, error) {
	var netM = new(NetManger)
	netM.cancelFunc = cancelFunc
	netM.idd = new(int64)
	netM.conn = make(map[int64]*NetC)
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
	netM.checkHeart = checkHeart
	netM.l = l
	netM.ws = false
	netM.connFailH = func(c *NetC) {
		chantypes.TryWriteChan(c.CloseC, true, false)
		netM.delConnByC(c)
		offlineh(c.Id)
	}
	recover_p.Go(netM.accept)
	netsM.listens = append(netsM.listens, netM)
	filelog.Log("监听成功:" + addr)
	return netM, nil
}

func (netM *NetManger) addConn(c *NetC) {
	netM.cLock.Lock()
	netM.conn[c.Id] = c
	netM.cLock.Unlock()
}

func (netM *NetManger) delConn(id int64) {
	netM.cLock.Lock()
	delete(netM.conn, id)
	netM.cLock.Unlock()
}

func (netM *NetManger) delConnByC(c *NetC) {
	if c.off {
		return
	}
	c.off = true
	netM.cLock.Lock()
	delete(netM.conn, c.Id)
	netM.cLock.Unlock()
}

func (netM *NetManger) getConn(id int64) *NetC {
	var res *NetC
	netM.cLock.RLock()
	res = netM.conn[id]
	netM.cLock.RUnlock()
	return res
}

func (netM *NetManger) getRandomConn() *NetC {
	var res *NetC
	netM.cLock.RLock()
	for _, res = range netM.conn {
		break
	}
	netM.cLock.RUnlock()
	return res
}

func (netM *NetManger) newConn(conn net.Conn) *NetC {
	res := netM.newNetC()

	res.conn = conn
	res.parseH = parseData
	res.readData = res.readTcpData
	res.sendData = res.sendTcpData
	res.close = res.closeTcp

	netM.addConn(res)
	return res
}

func (netM *NetManger) accept() {
	for {
		c, e := netM.l.AcceptTCP()
		if e != nil {
			fmt.Println(e)
			return
		}
		filelog.Debug("建立新连接成功," + c.RemoteAddr().String())

		nconn := netM.newConn(c)
		netM.sucessH(nconn.Id)
		netM.startOneConn(nconn)
	}
}

// 登录成功，开始监听
func (netM *NetManger) startOneConn(nconn *NetC) {
	recover_p.Go(func() { netM.send(nconn, netM.cancelFunc) })
	recover_p.Go(func() { netM.recv(nconn, netM.cancelFunc) })
	if netM.checkHeart {
		recover_p.Go(nconn.checkTimeOut)
	}
}

func (nc *NetC) readTcpData() ([]byte, uint16, error) {
	var bhead = make([]byte, MaxLength)
	_, e := io.ReadFull(nc.conn, bhead)
	if e != nil {
		return nil, 0, e
	}

	length := binary.BigEndian.Uint32(bhead) - MaxLength
	if length > 2e5 || length < ProtoLen {
		return nil, 1, errors.New(fmt.Sprintln("data too big or little,length:"+strconv.Itoa(int(length)+MaxLength), " ordata:", bhead))
	}
	var data = make([]byte, length)
	_, e = io.ReadFull(nc.conn, data)
	if e != nil {
		return nil, 0, e
	}
	return data, 0, nil
}

func (nc *NetC) sendTcpData(data []byte) error {
	var _, err = nc.conn.Write(data)
	return err
}

func (nc *NetC) closeTcp() {
	nc.conn.Close()
}

func (netM *NetManger) send(c *NetC, ctx context.Context) {
	filelog.Debug("开启发送消息:", c.Id)
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.CloseC:
			netM.delConnByC(c)
			return
		case data := <-c.SendC:
			e := c.sendData(data)
			if e != nil {
				netM.connFailH(c)
				return
			}
			if !c.lastHeart {
				c.lastHeart = true
			}
		}
	}
}

func (netM *NetManger) recv(c *NetC, ctx context.Context) {
	filelog.Debug("开启监听消息:", c.Id)
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.CloseC:
			netM.delConnByC(c)
			return
		default:

		}
		data, id, err := c.readData()
		if err != nil {
			if id != 0 {
				filelog.Error("read err:", err)
			}
			filelog.Log("连接断开:", c.Id)
			netM.connFailH(c)
			return
		}

		if !c.lastHeart {
			c.lastHeart = true
		}

		protoId := binary.BigEndian.Uint16(data[:2])
		if protoId == HEART { //心跳
			continue
		} //else if protoId == SYNCMSG { //同步消息
		//	handleSyncMsg(data[2:])
		//}
		netM.handle(data, c.Id)
	}
}

func (netM *NetManger) GetConnAddr(id int64) string {
	conn := netM.getConn(id)
	if conn == nil {
		return ""
	} else {
		return conn.conn.RemoteAddr().String()
	}
}

func (netM *NetManger) GetRandomConnAddr() string {
	conn := netM.getRandomConn()
	if conn == nil {
		return ""
	} else {
		return conn.conn.RemoteAddr().String()
	}
}
