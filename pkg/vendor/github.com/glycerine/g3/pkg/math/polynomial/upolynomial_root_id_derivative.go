package polynomial

// Source: upolynomial.cpp:351-363, reached through get_root_id's Sturm prefix.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// The source multiplier is plain RAW, with no lexical destructor. Preserve an
// actually allocated multiplier when the next iteration declares another one.
type upolySturmDerivativeRaw struct {
	value  unumeral.Int
	next   *upolySturmDerivativeRaw
	header uint64
}

func (q *upolyRootIDScope) retireDerivativeMultiplier() error {
	if q.context().rootIDFrame(q.owner.config.Integer) != q {
		return ErrUPolynomialOwner
	}
	if q.derivative.NativeCellToken() == 0 {
		return nil
	}
	n := uint64(reflect.TypeOf(upolySturmDerivativeRaw{}).Size())
	if e := q.owner.config.Integer.Budget().Reserve(n); e != nil {
		return e
	}
	q.derivativeRetired = &upolySturmDerivativeRaw{value: q.derivative, next: q.derivativeRetired, header: n}
	q.derivative = unumeral.Int{}
	return nil
}

func (q *upolyRootIDScope) cleanupDerivativeMultiplier() error {
	z := q.owner.config.Integer
	if e := z.CleanupRawNumeral(&q.derivative); e != nil {
		return e
	}
	for q.derivativeRetired != nil {
		r := q.derivativeRetired
		if e := z.CleanupRawNumeral(&r.value); e != nil {
			return e
		}
		z.Budget().Release(r.header)
		r.header = 0
		q.derivativeRetired = r.next
	}
	return nil
}
