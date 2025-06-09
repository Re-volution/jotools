package net

import (
	"time"
	"ysj/jotools/filelog"
)

// 检测心跳超时
func (nc *NetC) checkTimeOut() {
	filelog.Debug("启动心跳检测:", nc.Id)
	for {
		if nc == nil {
			return
		}

		if !nc.lastHeart || nc.off {
			if !nc.off {
				nc.off = true
				if nc.connws != nil {
					nc.connws.Close()
				} else {
					nc.conn.Close()
				}
				filelog.Error("conn no heart,exit:", nc.Id)
			}
			return
		}
		nc.lastHeart = false
		time.Sleep(62 * time.Second)
	}
}
