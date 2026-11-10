package jconfig

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
)

func ReadConfig(filename string, dst interface{}) error {
	if dst == nil {
		return errors.New("输入值必须是指针")
	}
	switch reflect.TypeOf(dst).Kind() {
	case reflect.Pointer:
	default:
		return errors.New("输入值必须是指针")
	}
	f, e := os.Open(filename)
	if e != nil {
		return e
	}
	defer f.Close()

	b, e := io.ReadAll(f)
	if e != nil {
		return e
	}

	e = json.Unmarshal(b, dst)
	if e != nil {
		return e
	}

	return nil
}
