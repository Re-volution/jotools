package net

import (
	"context"
	"encoding/binary"
	"github.com/Re-volution/ltime"
	"github.com/gorilla/websocket"
	"net"
	"sync/atomic"
	"time"
	"ysj/jotools/chantypes"
	"ysj/jotools/filelog"
	"ysj/jotools/recover_p"
)

type ConnManger struct {
	*NetC
	Addr    string                        //重连地址
	handle  func([]byte, int64)           //消息处理
	succ    func(...interface{}) int      //连接成功回调
	offline func(c *NetC, rech chan bool) //断线回调
	reConns chan bool
}

func (cm *ConnManger) newCM() *NetC {
	var res = new(NetC)
	res.Id = atomic.AddInt64(unionId, 1)
	res.SendC = make(chan []byte, 1024)
	res.CloseC = make(chan bool, 1)
	res.lastHeart = true
	return res
}

func (cm *ConnManger) newConn(conn net.Conn) *NetC {
	res := cm.newCM()

	res.conn = conn
	res.parseH = parseData
	res.readData = res.readTcpData
	res.close = res.closeTcp
	res.sendData = res.sendTcpData

	cm.NetC = res
	return res
}

func (cm *ConnManger) newConnWs(conn *websocket.Conn) *NetC {
	res := cm.newCM()

	res.connws = conn
	res.parseH = parseWsData
	res.readData = res.readWsData
	res.close = res.closeWs
	res.sendData = res.sendWsData

	cm.NetC = res
	return res
}

func (cm *ConnManger) conn(cxt context.Context) {
	conn, err := net.Dial("tcp", cm.Addr)
	if err != nil {
		filelog.Error("Dial fail ,err:", err, " addr:", cm.Addr)
		chantypes.TryWriteChan(cm.reConns, true, true)
		return
	}
	filelog.Log("连接成功,", cm.Addr)

	nconn := cm.newConn(conn)
	cm.startOneConn(nconn, cxt)
	return
}

func (cm *ConnManger) connWs(cxt context.Context) {
	conn, _, err := websocket.DefaultDialer.Dial(cm.Addr, nil)
	if err != nil {
		filelog.Error("Dial fail ,err:", err, " addr:", cm.Addr)
		chantypes.TryWriteChan(cm.reConns, true, true)
		return
	}
	filelog.Log("连接成功,", cm.Addr)

	nconn := cm.newConnWs(conn)
	cm.startOneConn(nconn, cxt)
	return
}

func (cm *ConnManger) startOneConn(nconn *NetC, cxt context.Context) {
	cm.succ(nconn.Id)
	recover_p.Go(func() { cm.send(cm.NetC, cxt) })
	recover_p.Go(func() { cm.recv(cm.NetC, cxt) })

	//TODO:发送心跳
}

func (cm *ConnManger) send(c *NetC, ctx context.Context) {
	filelog.Debug("开启发送消息:", c.Id)
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.CloseC:
			return
		case data := <-c.SendC:
			e := c.sendData(data)
			if e != nil {
				cm.offline(c, cm.reConns)
				return
			}
			if !c.lastHeart {
				c.lastHeart = true
			}
		}
	}
}

func (cm *ConnManger) recv(c *NetC, ctx context.Context) {
	filelog.Debug("开启监听消息:", c.Id)
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.CloseC:
			return
		default:
		}
		data, id, err := cm.NetC.readData()
		if err != nil {
			if id != 0 {
				filelog.Error("read err:", err)
			}
			filelog.Log("连接断开:", cm.Id)
			cm.offline(cm.NetC, cm.reConns)
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
		cm.handle(data, c.Id)
	}
}

var unionId = new(int64)

func Conn(addr string, h func([]byte, int64), offlineh func(), successh func(...interface{}) int, cxt context.Context) *ConnManger {
	var cm = new(ConnManger)
	cm.reConns = make(chan bool, 1)
	cm.handle = h
	cm.succ = successh
	cm.Addr = addr
	cm.offline = func(c *NetC, rech chan bool) {
		chantypes.TryWriteChan(c.CloseC, true, false)
		chantypes.TryWriteChan(rech, true, true)
		offlineh()
	}

	recover_p.Go(func() {
		cm.reConn(cxt)
	})
	netsM.conns = append(netsM.conns, cm)
	cm.conn(cxt)
	return cm
}

func (cm *ConnManger) reConn(cxt context.Context) {
	var retime int64
	for {
		select {
		case <-cxt.Done():
			filelog.Log("重连系统关闭")
			return
		case <-cm.reConns:
			retime = ltime.GetS() + 3
		default:
			time.Sleep(5 * time.Second)
		}

		if retime == 0 {
			continue
		}

		if ltime.GetS() > retime {
			filelog.Log("正在重连:" + cm.Addr)
			cm.conn(cxt)
			retime = 0
		}
		time.Sleep(3 * time.Second)
	}
}
