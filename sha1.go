package jotools

import (
	"crypto/sha1"
	"fmt"
)

var salt = "aiyunyou2024"

func Sha1(str string) string {
	t := sha1.New()
	t.Write([]byte(str + salt))
	return fmt.Sprintf("%x", t.Sum(nil))
}
