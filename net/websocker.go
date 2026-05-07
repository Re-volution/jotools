package net

import (
	"context"
	"errors"
	"github.com/Re-volution/jotools/chantypes"
	"github.com/Re-volution/jotools/filelog"
	"github.com/Re-volution/jotools/recover_p"
	"github.com/gorilla/websocket"
	"net/http"
	"sync"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// 启动 websocket 监听
func StartWSListen(wsHandleF string, port string, h func([]byte, int64), offlineh func(nid int64), successh func(...interface{}) int, cancelFunc context.Context, checkHeart bool) (*NetManger, error) {
	var netM = new(NetManger)
	netM.cancelFunc = cancelFunc
	netM.conn = new(sync.Map)
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
