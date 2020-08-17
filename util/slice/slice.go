package slice

import (
	"kortho/util/curry"
	"kortho/util/functor"
	"kortho/util/typeclass"
)

func Nub(xs []typeclass.Ord) []typeclass.Ord {
	var rs []typeclass.Ord

	for {
		switch {
		case len(xs) == 0:
			return rs
		default:
			rs = append(rs, xs[0])
			xs = Filter(curry.NotEq(xs[0]), xs[1:])
		}
	}
}

func Filter(f func(typeclass.Ord) bool, xs []typeclass.Ord) []typeclass.Ord {
	var rs []typeclass.Ord

	for {
		switch {
		case len(xs) == 0:
			return rs
		case f(xs[0]):
			rs = append(rs, xs[0])
		}
		xs = xs[1:]
	}
}

func Elem(x typeclass.Ord, xs []typeclass.Ord) typeclass.Ord {
	for {
		switch {
		case len(xs) == 0:
			return nil
		case xs[0].Eq(x):
			return xs[0]
		default:
			xs = xs[1:]
		}
	}
}

func ElemIndex(x typeclass.Ord, xs []typeclass.Ord) int {
	for i, j := 0, len(xs); i < j; i++ {
		if xs[i].Eq(x) {
			return i
		}
	}
	return -1
}

func Delete(x typeclass.Ord, xs []typeclass.Ord) []typeclass.Ord {
	var rs []typeclass.Ord

	for {
		switch {
		case len(xs) == 0:
			return rs
		case xs[0].Eq(x):
			return append(rs, xs[1:]...)
		default:
			rs = append(rs, xs[0])
			xs = xs[1:]
		}
	}
}

func DeleteBy(f func(typeclass.Ord) bool, xs []typeclass.Ord) []typeclass.Ord {
	var rs []typeclass.Ord

	for {
		switch {
		case len(xs) == 0:
			return rs
		case f(xs[0]):
			return append(rs, xs[1:]...)
		default:
			rs = append(rs, xs[0])
			xs = xs[1:]
		}
	}
}

type sliceRange struct {
	x int
	y int
}

func Qsort(xs []typeclass.Ord) []typeclass.Ord {
	var qs []*sliceRange

	ys := make([]typeclass.Ord, len(xs))
	copy(ys, xs)
	qs = append(qs, &sliceRange{0, len(ys) - 1})
	for len(qs) > 0 {
		x, y := qs[0].x, qs[0].y
		if x < y {
			z := ys[x]
			ls := Filter(curry.Lt(z), ys[x+1:y+1])
			rs := Filter(curry.Ge(z), ys[x+1:y+1])
			if len(ls) != 0 {
				copy(ys[x:x+len(ls)], ls)
				qs = append(qs, &sliceRange{x, x + len(ls) - 1})
			}
			if len(rs) != 0 {
				copy(ys[x+len(ls)+1:y+1], rs)
				qs = append(qs, &sliceRange{x + len(ls) + 1, y})
			}
			ys[x+len(ls)] = z
		}
		qs = qs[1:]
	}
	return ys
}

func Bsearch(x typeclass.Ord, xs []typeclass.Ord) int {
	mid, start, end := 0, 0, len(xs)-1
	for start <= end {
		mid = start + (end-start)/2
		switch {
		case xs[mid].Eq(x):
			return mid
		case xs[mid].Lt(x):
			start = mid + 1
		default:
			end = mid - 1
		}
	}
	return -1
}

func Push(x typeclass.Ord, xs []typeclass.Ord) []typeclass.Ord {
	var rs []typeclass.Ord

	for {
		switch {
		case len(xs) == 0:
			return append(rs, x)
		case xs[0].Ge(x):
			rs = append(rs, x)
			return append(rs, xs...)
		default:
			rs = append(rs, xs[0])
			xs = xs[1:]
		}
	}
}

// 将一个列表根据f映射成另一个元素
func Map(f functor.MapFunc, xs []typeclass.Ord) []typeclass.Ord {
	switch {
	case len(xs) == 0:
		return xs
	default:
		return append([]typeclass.Ord{f(xs[0])}, Map(f, xs[1:])...)
	}
}

// 将一个列表根据x和f从左到右折叠成一个新的元素
func Foldl(f functor.FoldFunc, x interface{}, xs []typeclass.Ord) interface{} {
	switch {
	case len(xs) == 0:
		return x
	default:
		return Foldl(f, f(xs[0], x), xs[1:])
	}
}

// 将一个列表根据x和f从右到左折叠成一个新的元素
func Foldr(f functor.FoldFunc, x interface{}, xs []typeclass.Ord) interface{} {
	switch {
	case len(xs) == 0:
		return x
	default:
		return f(xs[0], Foldr(f, x, xs[1:]))
	}
}

func FoldWhile(f func(typeclass.Ord, interface{}) bool, x interface{}, xs []typeclass.Ord) interface{} {
	switch {
	case len(xs) == 0:
		return x
	case !f(xs[0], x):
		return x
	default:
		return FoldWhile(f, f(xs[0], x), xs[1:])
	}
}
