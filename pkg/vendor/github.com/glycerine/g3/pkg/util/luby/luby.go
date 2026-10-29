package luby

// Source: src/util/luby.cpp:21-31; src/util/luby.h:22-29.

import (
	"errors"
	"math"
)

var ErrDomain = errors.New("luby: source floating conversion outside uint32")

func unsigned(x float64) (uint32, error) {
	x = math.Trunc(x)
	if math.IsNaN(x) || x < 0 || x > float64(^uint32(0)) {
		return 0, ErrDomain
	}
	return uint32(x), nil
}

// Value returns the source's one-based Luby sequence value. Arithmetic keeps
// uint32 wraparound and the original floating log/floor/pow decisions.
func Value(i uint32) (uint32, error) {
	if i == 1 {
		return 1, nil
	}
	k := math.Log(float64(i+1)) / math.Log(2)
	if k == math.Floor(k+.5) {
		return unsigned(math.Pow(2, k-1))
	}
	v, err := unsigned(math.Floor(k))
	if err != nil {
		return 0, err
	}
	n, err := unsigned(math.Pow(2, float64(v)))
	if err != nil {
		return 0, err
	}
	next := i - n + 1
	if next == i {
		return 0, ErrDomain
	}
	return Value(next)
}
