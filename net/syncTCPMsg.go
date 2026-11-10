package net

import (
	"encoding/json"
	"github.com/Re-volution/jotools/filelog"
	"github.com/Re-volution/jotools/recover_p"
	"github.com/Re-volution/ltime"
	"sync"
	"sync/atomic"
	"time"
)

type syncTcpMsg struct {
	mu      sync.Mutex
	data    map[int64]*stmData
	ch2     chan *UMsg
	unionId *int64
}

type stmData struct {
	ch chan []byte
	Id int64
	ot int64
}

type UMsg struct {
	UUid int64  `json:"uid,omitempty"`
	Msg  []byte `json:"msg,omitempty"`
}

type WaitSendData interface {
	SetUid(int64)
}

var stmSyncOnce sync.Once
var stm *syncTcpMsg

// WaitMsg 发出请求并等待对端按 uid 回包。超时后通道收到 nil。
func (netM *NetManger) WaitMsg(protoId uint16, cId int64, data WaitSendData) chan []byte {
	stmSyncOnce.Do(startSync)
	ch := make(chan []byte, 1)
	uid := stm.addWait(ch)
	data.SetUid(uid)
	if cId == 0 {
		netM.SendRandC(data, protoId)
	} else {
		netM.SendById(data, cId, protoId)
	}
	return ch
}

func startSync() {
	stm = &syncTcpMsg{
		data:    make(map[int64]*stmData),
		ch2:     make(chan *UMsg, 1024),
		unionId: new(int64),
	}
	recover_p.Go(stm.run)
}

func (s *syncTcpMsg) addWait(ch chan []byte) int64 {
	uid := atomic.AddInt64(s.unionId, 1)
	s.mu.Lock()
	s.data[uid] = &stmData{ch: ch, Id: uid, ot: ltime.GetS() + 3}
	s.mu.Unlock()
	return uid
}

func (s *syncTcpMsg) run() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case d := <-s.ch2:
			if d != nil {
				s.deliver(d.UUid, d.Msg)
			}
		case <-ticker.C:
			s.expire(ltime.GetS())
		}
	}
}

func (s *syncTcpMsg) deliver(id int64, msg []byte) {
	s.mu.Lock()
	d := s.data[id]
	if d != nil {
		delete(s.data, id)
	}
	s.mu.Unlock()
	if d != nil {
		trySend(d.ch, msg)
	}
}

func (s *syncTcpMsg) expire(now int64) {
	s.mu.Lock()
	var expired []*stmData
	for id, d := range s.data {
		if d == nil || d.ot <= now {
			expired = append(expired, d)
			delete(s.data, id)
		}
	}
	s.mu.Unlock()
	for _, d := range expired {
		if d != nil {
			trySend(d.ch, nil)
		}
	}
}

func trySend(ch chan []byte, msg []byte) {
	if ch == nil {
		return
	}
	select {
	case ch <- msg:
	default:
	}
}

func handleSyncMsg(data []byte) {
	if stm == nil {
		filelog.Error("sync msg before init")
		return
	}
	t := new(UMsg)
	if err := json.Unmarshal(data, t); err != nil {
		filelog.Error("Unmarshal err:", err, "data:", string(data))
		return
	}
	select {
	case stm.ch2 <- t:
	default:
		filelog.Error("sync msg queue full:", t.UUid)
	}
}
