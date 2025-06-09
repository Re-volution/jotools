package chantypes

import (
	"runtime"
	"time"
	"ysj/jotools/filelog"
)

// TryWriteChan 防止协程卡死
// 参数 ch 待写入通道  bl 待写入数据 bo 是否在写入失败后记录
func TryWriteChan[T any](ch chan T, bl T, bo bool) bool {
	select {
	case ch <- bl:
		return true
	default:
		if bo {
			pc, fill, line, _ := runtime.Caller(1)
			filelog.Error("写入数据失败:", runtime.FuncForPC(pc).Name(), fill, line)
		}
		return false
	}
}

// 读取通道，设定超时时间
func ReadChanByOverTime[T any](ch chan T, t int) (*T, bool) {
	var tm = time.NewTimer(time.Second * time.Duration(t)) //如果使用频繁，性能可能有消耗，根据需要可以自行实现
	select {                                               //如果性能消耗，可以for循环 自行设置循环间隔 使用default 睡眠
	case <-tm.C:
		return nil, false
	case data := <-ch:
		tm.Stop()
		return &data, true
	}
}
