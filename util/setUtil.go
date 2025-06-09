package util

type Set[T comparable] map[T]bool

func NewSet[T comparable]() Set[T] {
	return make(Set[T])
}

func (s Set[T]) Add(value T) {
	// 添加集合元素
	s[value] = true
}

func (s Set[T]) Remove(value T) {
	// 删除集合元素
	delete(s, value)
}

func (s Set[T]) Contains(value T) bool {
	_, ok := s[value]
	return ok
}

func (s Set[T]) Difference(other Set[T]) Set[T] {
	// 求减集
	result := NewSet[T]()
	for value := range s {
		if !other.Contains(value) {
			result.Add(value)
		}
	}
	return result
}

func (s Set[T]) Intersection(other Set[T]) Set[T] {
	result := NewSet[T]()
	for value := range s {
		if other.Contains(value) {
			result.Add(value)
		}
	}
	return result
}

func (s Set[T]) Union(other Set[T]) Set[T] {
	result := NewSet[T]()
	for value := range s {
		result.Add(value)
	}
	for value := range other {
		result.Add(value)
	}
	return result
}

func (s Set[T]) ToArray() []T {
	result := make([]T, 0)
	for value := range s {
		result = append(result, value)
	}
	return result
}
