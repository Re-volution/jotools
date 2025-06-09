package net

import (
	"context"
	"errors"
	"github.com/gorilla/websocket"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"
	"sync/atomic"
	"ysj/jotools/chantypes"
	"ysj/jotools/filelog"
	"ysj/jotools/recover_p"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
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
		return
	}
	serid, _ := strconv.Atoi(ser[0])
	uid := query[0]
	if serid == 0 {
		filelog.Error("serid == 0 :", newValues)
		return
	}
	filelog.Debug("建立新连接成功,"+c.RemoteAddr().String(), " uid:", uid)
	if uid == "" {
		filelog.Error("uid can't \"\"")
		c.WriteMessage(1, []byte("登录失败"))
		return
	}
	nconn := netM.newConnWs(c)

	code := netM.sucessH(uid, nconn.Id, serid)
	if code != 0 {
		c.WriteMessage(1, []byte("登录失败"))
		filelog.Error("玩家登录失败", code)
		netM.delConnByC(nconn)
		return
	}
	netM.startOneConn(nconn)
}

// 启动 websocket 监听
func StartWSListen(wsHandleF string, port string, h func([]byte, int64), offlineh func(nid int64), successh func(...interface{}) int, cancelFunc context.Context, checkHeart bool) (*NetManger, error) {
	var netM = new(NetManger)
	netM.cancelFunc = cancelFunc
	netM.idd = new(int64)
	netM.conn = make(map[int64]*NetC)
	netM.handle = h
	netM.sucessH = successh
	netM.ws = true
	netM.checkHeart = checkHeart

	netM.connFailH = func(c *NetC) {
		chantypes.TryWriteChan(c.CloseC, true, false)
		netM.delConnByC(c)
		offlineh(c.Id)
	}
	recover_p.Go(func() { netM.acceptWS(wsHandleF, port) })
	netsM.wsNet = append(netsM.wsNet, netM)
	filelog.Log("监听成功:" + port)
	return netM, nil
}

func (netM *NetManger) newNetC() *NetC {
	var res = new(NetC)
	res.Id = atomic.AddInt64(netM.idd, 1)
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

func (netM *NetManger) acceptWS(wsHandleF, port string) {
	http.HandleFunc(wsHandleF, netM.EchoMessage)
	http.ListenAndServe(":"+port, nil)
}

func (c *NetC) sendWsData(data []byte) error {
	return c.connws.WriteMessage(websocket.BinaryMessage, data)
}
func (c *NetC) readWsData() ([]byte, uint16, error) {
	_, data, err := c.connws.ReadMessage()
	if err != nil {
		return data, 0, err
	}

	if len(data) < ProtoLen+WSIncLen {
		return nil, 1, errors.New("消息长度不对:" + string(data))
	}
	return data, 0, nil
}
func (c *NetC) closeWs() {
	c.connws.Close()
}
