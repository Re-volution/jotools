package net

import (
	"context"
	"github.com/Re-volution/jotools/chantypes"
	"github.com/Re-volution/jotools/filelog"
	"github.com/Re-volution/jotools/recover_p"
	"github.com/gorilla/websocket"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type ConnManger struct {
	*NetC
	Addr      string
	handle    func([]byte, int64)
	succ      func(...interface{}) int
	onOffline func()
	reConns   chan bool
	stopped   atomic.Bool
	mu        sync.Mutex
}

func (cm *ConnManger) newCM() *NetC {
	res := new(NetC)
	res.Id = atomic.AddInt64(unionId, 1)
	res.SendC = make(chan []byte, 1024)
	res.CloseC = make(chan bool, 1)
	res.touch()
	return res
}

func (cm *ConnManger) newConn(conn net.Conn) *NetC {
	res := cm.newCM()
	res.conn = conn
	res.parseH = parseData
	res.readData = res.readTcpData
	res.close = res.closeTcp
	res.sendData = res.sendTcpData
	res.onClose = cm.notifyOffline
	return res
}

func (cm *ConnManger) newConnWs(conn *websocket.Conn) *NetC {
	res := cm.newCM()
	res.connws = conn
	res.parseH = parseWsData
	res.readData = res.readWsData
	res.close = res.closeWs
	res.sendData = res.sendWsData
	res.onClose = cm.notifyOffline
	return res
}

func (cm *ConnManger) publish(nconn *NetC) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if cm.stopped.Load() {
		return false
	}
	cm.NetC = nconn
	return true
}

func (cm *ConnManger) current() *NetC {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.NetC
}

func (cm *ConnManger) stop() {
	cm.mu.Lock()
	cm.stopped.Store(true)
	nc := cm.NetC
	cm.mu.Unlock()
	if nc != nil {
		nc.shutdownQuiet()
	}
}

func (cm *ConnManger) notifyOffline() {
	if cm.stopped.Load() {
		return
	}
	chantypes.TryWriteChan(cm.reConns, true, true)
	if cm.onOffline != nil {
		cm.onOffline()
	}
}

func (cm *ConnManger) dial(ctx context.Context) {
	if cm.stopped.Load() || (ctx != nil && ctx.Err() != nil) {
		return
	}
	if strings.HasPrefix(cm.Addr, "ws://") || strings.HasPrefix(cm.Addr, "wss://") {
		cm.connWs(ctx)
		return
	}
	cm.conn(ctx)
}

func (cm *ConnManger) conn(ctx context.Context) {
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(contextOrBackground(ctx), "tcp", cm.Addr)
	if err != nil {
		filelog.Error("Dial fail ,err:", err, " addr:", cm.Addr)
		if !cm.stopped.Load() {
			chantypes.TryWriteChan(cm.reConns, true, true)
		}
		return
	}
	if cm.stopped.Load() || (ctx != nil && ctx.Err() != nil) {
		_ = conn.Close()
		return
	}
	setTCPKeepAlive(conn)
	filelog.Log("连接成功,", cm.Addr)
	nconn := cm.newConn(conn)
	if !cm.publish(nconn) {
		nconn.shutdownQuiet()
		return
	}
	cm.startOneConn(nconn, ctx)
}

func (cm *ConnManger) connWs(ctx context.Context) {
	dctx, cancel := context.WithTimeout(contextOrBackground(ctx), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.DefaultDialer.DialContext(dctx, cm.Addr, nil)
	if err != nil {
		filelog.Error("Dial fail ,err:", err, " addr:", cm.Addr)
		if !cm.stopped.Load() {
			chantypes.TryWriteChan(cm.reConns, true, true)
		}
		return
	}
	if cm.stopped.Load() || (ctx != nil && ctx.Err() != nil) {
		_ = conn.Close()
		return
	}
	filelog.Log("连接成功,", cm.Addr)
	nconn := cm.newConnWs(conn)
	if !cm.publish(nconn) {
		nconn.shutdownQuiet()
		return
	}
	cm.startOneConn(nconn, ctx)
}

func (cm *ConnManger) startOneConn(nconn *NetC, ctx context.Context) {
	if cm.succ != nil {
		cm.succ(nconn.Id)
	}
	recover_p.Go(func() { nconn.watchCtx(ctx) })
	recover_p.Go(func() { nconn.loopSend(ctx) })
	recover_p.Go(func() { nconn.loopRecv(ctx, cm.handle) })
	recover_p.Go(func() { nconn.heartSend(ctx) })
}

func (cm *ConnManger) Close() {
	if cm == nil {
		return
	}
	if nc := cm.current(); nc != nil {
		nc.shutdown()
	}
}

func contextOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

var unionId = new(int64)

func Conn(addr string, h func([]byte, int64), offlineh func(), successh func(...interface{}) int, ctx context.Context) *ConnManger {
	cm := new(ConnManger)
	cm.reConns = make(chan bool, 1)
	cm.handle = h
	cm.succ = successh
	cm.Addr = addr
	cm.onOffline = offlineh

	recover_p.Go(func() {
		cm.reConn(ctx)
	})
	netsM.mu.Lock()
	netsM.conns = append(netsM.conns, cm)
	netsM.mu.Unlock()
	cm.dial(ctx)
	return cm
}

func (cm *ConnManger) reConn(ctx context.Context) {
	var timer *time.Timer
	var timerC <-chan time.Time
	for {
		select {
		case <-ctxDone(ctx):
			if timer != nil {
				timer.Stop()
			}
			filelog.Log("重连系统关闭")
			return
		case <-cm.reConns:
			if cm.stopped.Load() {
				return
			}
			if timer == nil {
				timer = time.NewTimer(3 * time.Second)
			} else if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
				timer.Reset(3 * time.Second)
			} else {
				timer.Reset(3 * time.Second)
			}
			timerC = timer.C
		case <-timerC:
			timerC = nil
			if cm.stopped.Load() {
				return
			}
			filelog.Log("正在重连:" + cm.Addr)
			cm.dial(ctx)
		}
	}
}
