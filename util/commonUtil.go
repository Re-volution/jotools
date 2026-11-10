package util

func IntToBool(i int) bool {
	return i != 0
}

func CheckInArray[T comparable](arr []T, elem T) bool {
	for _, v := range arr {
		if v == elem {
			return true
		}
	}
	return false
}
