package net

import (
	"context"
	"errors"
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
	httpSrv    *http.Server
	ws         bool
	cancelFunc context.Context
	conn       *sync.Map
	idd        atomic.Int64
	addr       string
	handle     func([]byte, int64)
	onOffline  func(nid int64)
	sucessH    func(...interface{}) int
	checkHeart bool
	closed     atomic.Bool
}

var netsM *nets

func init() {
	netsM = new(nets)
}

func Stop() {
	netsM.mu.Lock()
	conns := append([]*ConnManger(nil), netsM.conns...)
	listens := append([]*NetManger(nil), netsM.listens...)
	wsNets := append([]*NetManger(nil), netsM.wsNet...)
	netsM.mu.Unlock()

	for _, cm := range conns {
		if cm == nil {
			continue
		}
		cm.stop()
	}
	for _, netM := range listens {
		if netM == nil {
			continue
		}
		if netM.l != nil {
			_ = netM.l.Close()
		}
		netM.closeConns()
	}
	for _, netM := range wsNets {
		if netM == nil {
			continue
		}
		if netM.httpSrv != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			_ = netM.httpSrv.Shutdown(ctx)
			cancel()
		}
		netM.closeConns()
	}
}

func (netM *NetManger) closeConns() {
	if netM == nil || netM.conn == nil {
		return
	}
	netM.closed.Store(true)
	var cs []*NetC
	netM.conn.Range(func(_, value any) bool {
		if c, ok := value.(*NetC); ok {
			cs = append(cs, c)
		}
		return true
	})
	for _, c := range cs {
		c.shutdown()
	}
}

func (netM *NetManger) addConn(c *NetC) bool {
	if netM.closed.Load() {
		return false
	}
	netM.conn.Store(c.Id, c)
	if netM.closed.Load() {
		netM.conn.Delete(c.Id)
		return false
	}
	return true
}

func (netM *NetManger) getConn(id int64) *NetC {
	temp, ok := netM.conn.Load(id)
	if !ok {
		return nil
	}
	c, _ := temp.(*NetC)
	return c
}

func (netM *NetManger) bindOffline(res *NetC) {
	res.onClose = func() {
		netM.conn.Delete(res.Id)
		if netM.onOffline != nil {
			netM.onOffline(res.Id)
		}
	}
}

func (netM *NetManger) newNetC() *NetC {
	res := new(NetC)
	res.Id = netM.idd.Add(1)
	res.SendC = make(chan []byte, 1024)
	res.CloseC = make(chan bool, 1)
	res.touch()
	return res
}

func (netM *NetManger) newConn(conn net.Conn) *NetC {
	res := netM.newNetC()
	res.conn = conn
	res.parseH = parseData
	res.readData = res.readTcpData
	res.sendData = res.sendTcpData
	res.close = res.closeTcp
	netM.bindOffline(res)
	if !netM.addConn(res) {
		res.shutdownQuiet()
		return nil
	}
	return res
}

func (netM *NetManger) newConnWs(c *websocket.Conn) *NetC {
	res := netM.newNetC()
	res.connws = c
	res.parseH = parseWsData
	res.readData = res.readWsData
	res.sendData = res.sendWsData
	res.close = res.closeWs
	netM.bindOffline(res)
	if !netM.addConn(res) {
		res.shutdownQuiet()
		return nil
	}
	return res
}

func StartTcpListen(addr string, h func([]byte, int64), offlineh func(nid int64), successh func(...interface{}) int, cancelFunc context.Context, checkHeart bool) (*NetManger, error) {
	tcpaddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}
	l, err := net.ListenTCP("tcp", tcpaddr)
	if err != nil {
		return nil, err
	}
	netM := new(NetManger)
	netM.cancelFunc = cancelFunc
	netM.conn = new(sync.Map)
	netM.handle = h
	netM.sucessH = successh
	netM.onOffline = offlineh
	netM.checkHeart = checkHeart
	netM.l = l
	netM.ws = false
	if cancelFunc != nil {
		go func() {
			<-cancelFunc.Done()
			_ = l.Close()
		}()
	}
	recover_p.Go(netM.accept)
	netsM.mu.Lock()
	netsM.listens = append(netsM.listens, netM)
	netsM.mu.Unlock()
	filelog.Log("监听成功:" + addr)
	return netM, nil
}

func (netM *NetManger) accept() {
	for {
		c, err := netM.l.AcceptTCP()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || (netM.cancelFunc != nil && netM.cancelFunc.Err() != nil) {
				return
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				filelog.Error("accept temporary:", err)
				time.Sleep(50 * time.Millisecond)
				continue
			}
			filelog.Error("accept tcp:", err)
			return
		}
		setTCPKeepAlive(c)
		filelog.Debug("建立新连接成功," + c.RemoteAddr().String())
		nconn := netM.newConn(c)
		if nconn == nil {
			continue
		}
		if netM.sucessH != nil {
			netM.sucessH(nconn.Id)
		}
		netM.startOneConn(nconn)
	}
}

func (netM *NetManger) startOneConn(nconn *NetC) {
	ctx := netM.cancelFunc
	recover_p.Go(func() { nconn.watchCtx(ctx) })
	recover_p.Go(func() { nconn.loopSend(ctx) })
	recover_p.Go(func() { nconn.loopRecv(ctx, netM.handle) })
	if netM.checkHeart {
		recover_p.Go(nconn.checkTimeOut)
	}
}

func (netM *NetManger) GetConnAddr(id int64) string {
	conn := netM.getConn(id)
	if conn == nil {
		return ""
	}
	return conn.remoteAddr()
}

func (netM *NetManger) GetRandomConnAddr() string {
	conn := netM.getRandomConn()
	if conn == nil {
		return ""
	}
	return conn.remoteAddr()
}

func (netM *NetManger) getRandomConn() *NetC {
	var res *NetC
	netM.conn.Range(func(_, value any) bool {
		res, _ = value.(*NetC)
		return false
	})
	return res
}

func (netM *NetManger) EchoMessage(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if e := recover(); e != nil {
			filelog.Error("EchoMessage panic:", e, string(debug.Stack()))
		}
	}()
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		filelog.Error("Upgrade errorCode:", err)
		return
	}
	started := false
	defer func() {
		if !started {
			_ = c.Close()
		}
	}()
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		filelog.Error("ParseQuery err:", err)
		return
	}
	query := values["Uid"]
	ser := values["Serverid"]
	if len(query) == 0 {
		query = r.Header["Uid"]
		ser = r.Header["Serverid"]
	}
	if len(query) == 0 || len(ser) == 0 {
		filelog.Error("len(query) == 0 || len(ser) == 0 :", r.Header)
		return
	}
	serid, err := strconv.Atoi(ser[0])
	if err != nil || serid == 0 {
		filelog.Error("serid invalid:", ser[0], err)
		return
	}
	uid := query[0]
	filelog.Debug("建立新连接成功,"+c.RemoteAddr().String(), " uid:", uid)
	if uid == "" {
		filelog.Error("uid can't \"\"")
		_ = c.WriteMessage(websocket.TextMessage, []byte("登录失败"))
		return
	}
	nconn := netM.newConnWs(c)
	if nconn == nil {
		return
	}
	code := 0
	if netM.sucessH != nil {
		code = netM.sucessH(uid, nconn.Id, serid)
	}
	if code != 0 {
		_ = c.WriteMessage(websocket.TextMessage, []byte("登录失败"))
		filelog.Error("玩家登录失败", code)
		netM.conn.Delete(nconn.Id)
		return
	}
	started = true
	netM.startOneConn(nconn)
}

func (netM *NetManger) String() string {
	return fmt.Sprintf("conn totle:%d,is ws:%t", netM.connCount(), netM.ws)
}

func (netM *NetManger) connCount() int {
	count := 0
	if netM.conn == nil {
		return 0
	}
	netM.conn.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}
