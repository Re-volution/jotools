package util

func IntToBool(i int) bool {
	return i != 0
}

func CheckInArray[T comparable](arr []T, elem T) bool {
	var inFlag bool
	for _, v := range arr {
		if v == elem {
			inFlag = true
		}
	}
	return inFlag
}
