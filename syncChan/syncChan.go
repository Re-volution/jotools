package syncChan

import (
	"github.com/Re-volution/ltime"
	"sync"
	"time"
	"ysj/jotools/chantypes"
	"ysj/jotools/filelog"
)

func init() {
	SyncMsg = new(sMsgC)
	SyncMsg.ch = make(chan *UMsg, 1024)
	SyncMsg.data = make(map[int64]*overTC)
	go func() {
		SyncMsg.handle()
	}()
}

var SyncMsg *sMsgC

type UMsg struct {
	UUid int64  `json:"uid,omitempty"`
	Msg  []byte `json:"msg,omitempty"`
}

type sMsgC struct {
	data map[int64]*overTC
	ch   chan *UMsg
	sy   sync.RWMutex
}

type overTC struct {
	ch chan []byte //消息返回通道
	ot int64       //超时时间
}

func (smg *sMsgC) getC(uid int64) chan []byte {
	var c *overTC
	smg.sy.RLock()
	c = smg.data[uid]
	smg.sy.RUnlock()
	if c != nil {
		return c.ch
	}
	return nil
}

// 同步通道主循环
func (smg *sMsgC) handle() {
	var nextT = ltime.GetS() + 5
	for {
		select {
		case data := <-SyncMsg.ch:
			filelog.Debug("同步消息返回:", data.UUid)
			c := smg.getC(data.UUid)
			if c != nil {
				c <- data.Msg
			}
			delete(smg.data, data.UUid)
		default:
			time.Sleep(20 * time.Millisecond)
		}
		var now = ltime.GetS() //处理超时
		if nextT < now {
			nextT = now + 5
			var waitdelC []int64
			smg.sy.Lock()
			for k, v := range smg.data {
				if v != nil {
					if v.ot < now {
						waitdelC = append(waitdelC, k)
						chantypes.TryWriteChan(v.ch, nil, false)
					}
				} else {
					waitdelC = append(waitdelC, k)
				}
			}
			for _, v := range waitdelC {
				filelog.Error("超时异步请求:", v)
				delete(smg.data, v)
			}
			smg.sy.Unlock()

		}
	}
}

func (smg *sMsgC) Add(uuid int64, c chan []byte) {
	filelog.Debug("同步消息添加监听:", uuid)
	smg.sy.Lock()
	smg.data[uuid] = &overTC{c, ltime.GetS() + 5}
	smg.sy.Unlock()
}

func (smg *sMsgC) ToCh(uuid int64, data []byte) {
	filelog.Debug("同步消息接收:", uuid)
	var msg = new(UMsg)
	msg.UUid = uuid
	msg.Msg = data
	chantypes.TryWriteChan(smg.ch, msg, true)
}
