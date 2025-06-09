package dencode

import (
	"encoding/json"
	"google.golang.org/protobuf/proto"
)

func Marshal(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

func Unmarshal[T any](data []byte) (*T, error) {
	var res = new(T)
	err := json.Unmarshal(data, res)
	return res, err
}

func MarshalProto(data proto.Message) ([]byte, error) {
	return proto.Marshal(data)
}

func UnmarshalProto[T proto.Message](msg T, data []byte) error {
	err := proto.Unmarshal(data, msg)
	return err
}
