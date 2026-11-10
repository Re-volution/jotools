package net

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/Re-volution/jotools/filelog"
	"github.com/Re-volution/jotools/recover_p"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"net"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
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
	conn       *sync.Map
	idd        atomic.Int64
	addr       string //链接地址
	handle     func([]byte, int64)
	connFailH  func(*NetC)              //连接断开的处理
	sucessH    func(...interface{}) int //连接成功做的处理.
	checkHeart bool                     //检测心跳
	httpSrv    *http.Server
	closed     bool
}

var netsM *nets

func init() {
	netsM = new(nets)
}

func Stop() {
	closeConns := func(netM *NetManger) {
		netM.closed = true
		if netM.l != nil {
			netM.l.Close()
		}
		if netM.httpSrv != nil {
			netM.httpSrv.Close()
		}
		netM.conn.Range(func(key, value interface{}) bool {
			if conn, ok := value.(*NetC); ok {
				conn.Close()
			}
			return true
		})
	}
	netsM.mu.Lock()
	conns := append([]*ConnManger{}, netsM.conns...)
	listens := append([]*NetManger{}, netsM.listens...)
	wsNet := append([]*NetManger{}, netsM.wsNet...)
	netsM.mu.Unlock()

	for _, connM := range conns {
		connM.stop = true
		if connM.NetC != nil {
			connM.NetC.Close()
		}
	}

	for _, netM := range wsNet {
		closeConns(netM)
	}
	for _, netM := range listens {
		closeConns(netM)
	}

}

func (netM *NetManger) addConn(c *NetC) {
	if netM.closed {
		c.Close()
		return
	}
	netM.conn.Store(c.Id, c)
}

func (netM *NetManger) delConn(id int64) {
	netM.conn.Delete(id)
}

func (netM *NetManger) delConnByC(c *NetC) {
	c.off = true
	netM.conn.Delete(c.Id)
}

func (netM *NetManger) getConn(id int64) *NetC {
	temp, ok := netM.conn.Load(id)
	if ok {
		return temp.(*NetC)
	} else {
		return nil
	}
}

func (netM *NetManger) getRandomConn() *NetC {
	var res *NetC
	netM.conn.Range(func(key, value interface{}) bool {
		res = value.(*NetC)
		return false // 立即停止，取第一个
	})

	return res
}

func (netM *NetManger) newConn(conn net.Conn) *NetC {
	res := netM.newNetC()

	res.conn = conn
	if tcp, ok := conn.(*net.TCPConn); ok {
		tcp.SetKeepAlive(true)
		tcp.SetKeepAlivePeriod(30 * time.Second)
	}
	res.parseH = parseData
	res.readData = res.readTcpData
	res.sendData = res.sendTcpData
	res.close = res.closeTcp

	netM.addConn(res)
	return res
}

func (netM *NetManger) accept() {
	for {
		if netM.cancelFunc != nil {
			select {
			case <-netM.cancelFunc.Done():
				netM.l.Close()
				return
			default:
			}
		}
		c, e := netM.l.AcceptTCP()
		if e != nil {
			if ne, ok := e.(net.Error); ok && ne.Timeout() {
				continue
			}
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
	if netM.cancelFunc != nil {
		recover_p.Go(func() {
			<-netM.cancelFunc.Done()
			nconn.Close()
		})
	}
	if netM.checkHeart {
		recover_p.Go(nconn.checkTimeOut)
	}
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
				if c.off {
					return
				}
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
			if !c.off {
				filelog.Log("连接断开:", c.Id)
			}
			netM.connFailH(c)
			return
		}

		if !c.lastHeart {
			c.lastHeart = true
		}

		if len(data) < ProtoLen {
			filelog.Error("read err:", "short data")
			netM.connFailH(c)
			return
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
	} else if conn.conn != nil {
		return conn.conn.RemoteAddr().String()
	} else if conn.connws != nil {
		return conn.connws.RemoteAddr().String()
	}
	return ""
}

func (netM *NetManger) GetRandomConnAddr() string {
	conn := netM.getRandomConn()
	if conn == nil {
		return ""
	} else if conn.conn != nil {
		return conn.conn.RemoteAddr().String()
	} else if conn.connws != nil {
		return conn.connws.RemoteAddr().String()
	}
	return ""
}

func (netM *NetManger) EchoMessage(w http.ResponseWriter, r *http.Request) {
	defer func() {
		e := recover()
		if e != nil {
			filelog.Error("EchoMessage panic:", string(debug.Stack()))
		}
	}()
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		filelog.Error("Upgrade errorCode:", err)
		return
	}
	newValues, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		filelog.Error("ParseQuery err:", err)
		c.Close()
		return
	}
	query := newValues["Uid"]
	ser := newValues["Serverid"]

	if len(query) == 0 {
		query = r.Header["Uid"]
		ser = r.Header["Serverid"]
	}
	if len(query) == 0 || len(ser) == 0 {
		filelog.Error("len(query) == 0 || len(ser) == 0 :", r.Header)
		c.Close()
		return
	}
	serid, _ := strconv.Atoi(ser[0])
	uid := query[0]
	if serid == 0 {
		filelog.Error("serid == 0 :", newValues)
		c.Close()
		return
	}
	filelog.Debug("建立新连接成功,"+c.RemoteAddr().String(), " uid:", uid)
	if uid == "" {
		filelog.Error("uid can't \"\"")
		c.WriteMessage(1, []byte("登录失败"))
		c.Close()
		return
	}
	nconn := netM.newConnWs(c)

	code := netM.sucessH(uid, nconn.Id, serid)
	if code != 0 {
		c.WriteMessage(1, []byte("登录失败"))
		filelog.Error("玩家登录失败", code)
		netM.delConnByC(nconn)
		nconn.Close()
		return
	}
	netM.startOneConn(nconn)
}

func (netM *NetManger) newNetC() *NetC {
	var res = new(NetC)
	res.Id = netM.idd.Add(1)
	res.SendC = make(chan []byte, 1024)
	res.CloseC = make(chan bool, 1)
	res.lastHeart = true
	return res
}

func (netM *NetManger) newConnWs(c *websocket.Conn) *NetC {
	res := netM.newNetC()

	res.connws = c
	res.parseH = parseWsData
	res.readData = res.readWsData
	res.sendData = res.sendWsData
	res.close = res.closeWs

	netM.addConn(res)
	return res
}
