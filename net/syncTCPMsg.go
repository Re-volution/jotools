package net

import (
	"encoding/json"
	"github.com/Re-volution/ltime"
	"sync"
	"sync/atomic"
	"time"
	"ysj/jotools/chantypes"
	"ysj/jotools/filelog"
	"ysj/jotools/recover_p"
)

type syncTcpMsg struct {
	data    map[int64]*stmData
	ch      chan *stmData
	ch2     chan *UMsg
	unionId *int64
}

type stmData struct {
	ch chan []byte //写出通道
	Id int64       //唯一id
	ot int64       //超时时间
}

type UMsg struct {
	UUid int64  `json:"uid,omitempty"`
	Msg  []byte `json:"msg,omitempty"`
}

type WaitSendData interface {
	SetUid(int64)
}

var stmSyncOnce = new(sync.Once)

var stm *syncTcpMsg

// TODO:需要和conn的发送与接收相结合 length+固定协议号+唯一id+普通协议号+数据
func (netM *NetManger) WaitMsg(protoId uint16, cId int64, data WaitSendData) chan []byte {
	stmSyncOnce.Do(run)
	uid := stm.getUnionId()
	var ch = make(chan []byte)
	stm.ch <- &stmData{ch, uid, ltime.GetS() + 3}
	data.SetUid(uid)
	if cId == 0 {
		netM.SendRandC(data, protoId)
	} else {
		netM.SendById(data, cId, protoId)
	}
	return ch
}

func run() {
	stm = new(syncTcpMsg)
	stm.ch = make(chan *stmData, 1024)
	stm.unionId = new(int64)
	stm.data = make(map[int64]*stmData)
	stm.ch2 = make(chan *UMsg, 1024)
	recover_p.Go(stm.run)
}
func (stm *syncTcpMsg) getUnionId() int64 {
	return atomic.AddInt64(stm.unionId, 1)
}

func (stm *syncTcpMsg) run() {
	var nextCheckT int64
	for {
		select {
		case d := <-stm.ch:
			stm.data[d.Id] = d
		case d := <-stm.ch2:
			ch2 := stm.data[d.UUid]
			if ch2 != nil {
				ch2.ch <- d.Msg
			}
		default:
			time.Sleep(5 * time.Millisecond)
		}
		now := ltime.GetS()
		if nextCheckT < now {
			nextCheckT = now + 1
			var wDelIds []int64
			for k, v := range stm.data {
				if v.ot < now {
					chantypes.TryWriteChan(v.ch, nil, false)
					wDelIds = append(wDelIds, k)
				}
			}
			for _, v := range wDelIds {
				delete(stm.data, v)
			}
		}

	}
}

func handleSyncMsg(data []byte) {
	var t = new(UMsg)
	e := json.Unmarshal(data, t)
	if e != nil {
		filelog.Error("Unmarshal err:", e, "data:", string(data))
		return
	}
	chantypes.TryWriteChan(stm.ch2, t, true)
}
