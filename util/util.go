package util

const (
	MINUINT64 = uint64(0)
	MAXUINT64 = ^MINUINT64
)

// Uint64AddOverflow 多个uint64相加溢出返回true,否则返回false
func Uint64AddOverflow(a uint64, b ...uint64) bool {
	for _, v := range b {
		if MAXUINT64-a < v {
			return true
		}
		a += v
	}
	return false
}

//Uint64SubOverflow 多个uint64相减如果溢出返回true，否则返回false
func Uint64SubOverflow(a uint64, b ...uint64) bool {
	for _, v := range b {
		if a < v {
			return true
		}
		a -= v
	}
	return false
}
