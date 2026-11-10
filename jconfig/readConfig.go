package jconfig

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
)

func ReadConfig(filename string, dst interface{}) error {
	if dst == nil || reflect.TypeOf(dst).Kind() != reflect.Pointer {
		return errors.New("输入值必须是指针")
	}
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}
