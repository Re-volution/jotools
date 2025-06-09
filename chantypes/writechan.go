package chantypes

import (
	"runtime"
	"wb3/jotools/filelog"
)

// TryWriteChan 防止协程卡死
// 参数 ch 待写入通道  bl 待写入数据 bo 是否在写入失败后记录
func TryWriteChan[T any](ch chan T, bl T, bo bool) {
	select {
	case ch <- bl:
	default:
		if bo {
			pc, fill, line, _ := runtime.Caller(1)
			filelog.Error("写入数据失败:", runtime.FuncForPC(pc).Name(), fill, line)
		}
	}
}
