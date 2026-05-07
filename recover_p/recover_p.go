package recover_p

import (
	"github.com/Re-volution/jotools/filelog"
	"reflect"
	"runtime/debug"
)

func Go(f func()) {
	go func() {
		defer func() {
			err := recover()
			if err != nil {
				filelog.Error("painc:", err, "\r\n", string(debug.Stack()), reflect.TypeOf(f).Name())
			}
		}()
		f()
	}()
}
