package syncChan

import (
	"github.com/Re-volution/jotools/filelog"
	"github.com/Re-volution/jotools/recover_p"
	"github.com/Re-volution/ltime"
	"sync"
	"time"
)

func init() {
	SyncMsg = new(sMsgC)
	SyncMsg.ch = make(chan *UMsg, 1024)
	SyncMsg.data = make(map[int64]*overTC)
	recover_p.Go(func() {
		SyncMsg.handle()
	})
}

var SyncMsg *sMsgC

type UMsg struct {
	UUid int64  `json:"uid,omitempty"`
	Msg  []byte `json:"msg,omitempty"`
}

type sMsgC struct {
	data map[int64]*overTC
	ch   chan *UMsg
	sy   sync.Mutex
}

type overTC struct {
	ch chan []byte
	ot int64
}

func (smg *sMsgC) handle() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case data := <-smg.ch:
			if data == nil {
				continue
			}
			filelog.Debug("同步消息返回:", data.UUid)
			smg.sy.Lock()
			c := smg.data[data.UUid]
			delete(smg.data, data.UUid)
			smg.sy.Unlock()
			if c != nil {
				writeWithTimeout(c.ch, data.Msg, data.UUid)
			}
		case <-ticker.C:
			smg.expire(ltime.GetS())
		}
	}
}

func (smg *sMsgC) expire(now int64) {
	var expired []struct {
		id int64
		ch chan []byte
	}
	smg.sy.Lock()
	for k, v := range smg.data {
		if v == nil || v.ot <= now {
			if v != nil {
				expired = append(expired, struct {
					id int64
					ch chan []byte
				}{k, v.ch})
			}
			delete(smg.data, k)
		}
	}
	smg.sy.Unlock()
	for _, v := range expired {
		filelog.Error("超时异步请求:", v.id)
		writeWithTimeout(v.ch, nil, v.id)
	}
}

func writeWithTimeout(ch chan []byte, data []byte, id int64) {
	if ch == nil {
		return
	}
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case ch <- data:
	case <-timer.C:
		filelog.Error("同步消息写出超时:", id)
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
	msg := &UMsg{UUid: uuid, Msg: data}
	select {
	case smg.ch <- msg:
	default:
		filelog.Error("同步消息队列已满:", uuid)
	}
}
