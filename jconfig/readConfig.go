package jconfig

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
)

func ReadConfig(filename string, dst interface{}) error {
	switch reflect.TypeOf(dst).Kind() {
	case reflect.Pointer:
	default:
		return errors.New("输入值必须是指针")
	}
	f, e := os.Open(filename)
	if e != nil {
		return e
	}

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
