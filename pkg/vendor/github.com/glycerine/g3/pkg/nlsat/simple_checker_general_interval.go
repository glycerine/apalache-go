package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:1156-1245.

import (
	"errors"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

func multiplySimpleInterval(am *polynomial.AlgebraicManager, a, b *simpleInterval) (out simpleInterval, err error) {
	products := make([]simpleEndpoint, 4)
	defer func() {
		for i := range products {
			err = errors.Join(err, products[i].close(am))
		}
	}()
	pairs := [][2]*simpleEndpoint{{&a.lower, &b.lower}, {&a.lower, &b.upper}, {&a.upper, &b.lower}, {&a.upper, &b.upper}}
	for i, pair := range pairs {
		if err = multiplySimpleEndpoint(am, pair[0], pair[1], &products[i]); err != nil {
			return out, err
		}
	}
	min, max := 0, 0
	for i := 1; i < 4; i++ {
		order, cause := products[i].compare(am, &products[min])
		if cause != nil {
			return out, cause
		}
		if order < 0 {
			min = i
		}
		order, cause = products[max].compare(am, &products[i])
		if cause != nil {
			return out, cause
		}
		if order < 0 {
			max = i
		}
	}
	out.lower.lower = true
	out.upper.lower = false
	if err = copySimpleEndpoint(am, &out.lower, &products[min]); err != nil {
		return out, err
	}
	out.lower.lower = true
	if err = copySimpleEndpoint(am, &out.upper, &products[max]); err != nil {
		return out, err
	}
	out.upper.lower = false
	return out, nil
}
func addSimpleInterval(am *polynomial.AlgebraicManager, a, b *simpleInterval) (out simpleInterval, err error) {
	out.lower.lower = true
	out.upper.lower = false
	for _, entry := range []struct{ dst, x, y *simpleEndpoint }{{&out.lower, &a.lower, &b.lower}, {&out.upper, &a.upper, &b.upper}} {
		if entry.x.infinite || entry.y.infinite {
			entry.dst.infinite = true
			entry.dst.open = true
			continue
		}
		entry.dst.open = entry.x.open || entry.y.open
		if err = am.Add(&entry.x.value, &entry.y.value, &entry.dst.value); err != nil {
			return out, err
		}
	}
	return out, nil
}
