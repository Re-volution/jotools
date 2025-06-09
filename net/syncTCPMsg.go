package net

import (
	"sync"
	"sync/atomic"
	"wb3/jotools/recover_p"
)

type syncTcpMsg struct {
	ch      chan *stmData
	unionId *int64
}

type stmData struct {
	ch chan []byte //写出通道
	Id int64       //唯一id
}

var stmSyncOnce = new(sync.Once)

var stm *syncTcpMsg

// TODO:需要和conn的发送与接收相结合 length+固定协议号+唯一id+普通协议号+数据
func WaitMsg(data []byte) chan []byte {
	stmSyncOnce.Do(run)
	var ch = make(chan []byte)
	stm.ch <- &stmData{ch, stm.getUnionId()}
	return ch
}

func run() {
	stm = new(syncTcpMsg)
	stm.ch = make(chan *stmData, 1024)
	stm.unionId = new(int64)
	recover_p.Go(stm.run)
}
func (stm *syncTcpMsg) getUnionId() int64 {
	return atomic.AddInt64(stm.unionId, 1)
}

func (stm *syncTcpMsg) run() {
	for {
		select {
		case <-stm.ch:
		}
	}
}
