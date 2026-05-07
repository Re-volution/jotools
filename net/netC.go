package net

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/Re-volution/jotools/chantypes"
	"github.com/Re-volution/jotools/filelog"
	"github.com/Re-volution/jotools/recover_p"
	"github.com/gorilla/websocket"
	"io"
	"net"
	"strconv"
)

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
