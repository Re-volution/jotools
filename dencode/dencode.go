package dencode

import "encoding/json"

func Marshal(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

func Unmarshal[T any](data []byte) (*T, error) {
	var res = new(T)
	err := json.Unmarshal(data, res)
	return res, err
}
