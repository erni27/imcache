package imcache

// Number is a constraint that permits any numeric type except complex ones.
type Number interface {
	~float32 | ~float64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~int | ~int8 | ~int16 | ~int32 | ~int64
}

// Increment increments the given number by one.
func Increment[V Number](old V) V {
	return old + 1
}

// Decrement decrements the given number by one.
func Decrement[V Number](old V) V {
	return old - 1
}
