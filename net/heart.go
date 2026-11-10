package net

import (
	"context"
	"github.com/Re-volution/jotools/chantypes"
	"github.com/Re-volution/jotools/filelog"
	"time"
)

const (
	heartTimeout  = 62 * time.Second
	heartInterval = 30 * time.Second
)

// checkTimeOut 在 heartTimeout 内没有任何收发则关闭连接。
// 用最后活跃时间判断，避免收发和检测互相覆盖。
func (nc *NetC) checkTimeOut() {
	if nc == nil {
		return
	}
	filelog.Debug("启动心跳检测:", nc.Id)
	nc.touch()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-nc.CloseC:
			return
		case now := <-ticker.C:
			if now.Unix()-nc.lastActive.Load() >= int64(heartTimeout/time.Second) {
				filelog.Error("conn no heart,exit:", nc.Id)
				nc.shutdown()
				return
			}
		}
	}
}

// heartSend 定时发送心跳。服务端只认收到的包，所以客户端必须主动发。
func (nc *NetC) heartSend(ctx context.Context) {
	if nc == nil {
		return
	}
	send := func() {
		if nc.off.Load() || nc.parseH == nil {
			return
		}
		data := nc.parseH(nil, HEART)
		if len(data) == 0 {
			return
		}
		chantypes.TryWriteChan(nc.SendC, data, false)
	}
	send()
	ticker := time.NewTicker(heartInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctxDone(ctx):
			return
		case <-nc.CloseC:
			return
		case <-ticker.C:
			send()
		}
	}
}
