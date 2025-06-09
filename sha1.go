package jotools

import (
	"crypto/sha1"
	"fmt"
)

var salt = ")534d1wa.xz"

func Sha1SetSalt(s string) {
	salt = s
}

func Sha1(str string) string {
	t := sha1.New()
	t.Write([]byte(str + salt))
	return fmt.Sprintf("%x", t.Sum(nil))
}
