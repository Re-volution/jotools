package net

import (
	"context"
	"errors"
	"github.com/Re-volution/jotools/filelog"
	"github.com/Re-volution/jotools/recover_p"
	"github.com/gorilla/websocket"
	"net"
	"net/http"
	"sync"
	"time"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// StartWSListen 启动 websocket 监听。端口被占用时直接返回错误。
func StartWSListen(wsHandleF string, port string, h func([]byte, int64), offlineh func(nid int64), successh func(...interface{}) int, cancelFunc context.Context, checkHeart bool) (*NetManger, error) {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return nil, err
	}
	netM := new(NetManger)
	netM.cancelFunc = cancelFunc
	netM.conn = new(sync.Map)
	netM.handle = h
	netM.sucessH = successh
	netM.onOffline = offlineh
	netM.ws = true
	netM.checkHeart = checkHeart

	mux := http.NewServeMux()
	mux.HandleFunc(wsHandleF, netM.EchoMessage)
	netM.httpSrv = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if cancelFunc != nil {
		go func() {
			<-cancelFunc.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = netM.httpSrv.Shutdown(ctx)
		}()
	}
	recover_p.Go(func() {
		serveErr := netM.httpSrv.Serve(ln)
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			filelog.Error("ws serve:", serveErr)
		}
	})
	netsM.mu.Lock()
	netsM.wsNet = append(netsM.wsNet, netM)
	netsM.mu.Unlock()
	filelog.Log("监听成功:" + port)
	return netM, nil
}

func (c *NetC) sendWsData(data []byte) error {
	if c.connws == nil {
		return errors.New("ws conn is nil")
	}
	return c.connws.WriteMessage(websocket.BinaryMessage, data)
}

func (c *NetC) readWsData() ([]byte, uint16, error) {
	_, data, err := c.connws.ReadMessage()
	if err != nil {
		return data, 0, err
	}
	if len(data) < ProtoLen {
		return nil, 1, errors.New("消息长度不对")
	}
	return data, 0, nil
}

func (c *NetC) closeWs() {
	if c.connws != nil {
		_ = c.connws.Close()
	}
}
