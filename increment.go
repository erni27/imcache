package imcache

type Number interface {
	~float32 | ~float64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~int | ~int8 | ~int16 | ~int32 | ~int64
}

func Increment[V Number](old V) V {
	return old + 1
}
