package recover_p

import (
	"github.com/Re-volution/jotools/filelog"
	"runtime"
	"runtime/debug"
)

func Go(f func()) {
	pc, file, line, _ := runtime.Caller(1)
	caller := ""
	if fn := runtime.FuncForPC(pc); fn != nil {
		caller = fn.Name()
	}
	go func() {
		defer func() {
			if err := recover(); err != nil {
				filelog.Error("panic:", err, " caller:", caller, " ", file, ":", line, "\r\n", string(debug.Stack()))
			}
		}()
		f()
	}()
}
