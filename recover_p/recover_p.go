package recover_p

import (
	"reflect"
	"runtime/debug"
	"wb3/jotools/filelog"
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
