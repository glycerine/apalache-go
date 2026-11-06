package polynomial

// Source: upolynomial.cpp:351-363; public Sturm/Fourier RAW multiplier.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

func (q *upolySequenceSturmScope) retireDerivativeMultiplier() error {
	if q.context().sequenceSturmFrame(q.owner.manager.config.Integer) != q {
		return ErrUPolynomialOwner
	}
	if q.derivative.NativeCellToken() == 0 {
		return nil
	}
	n := uint64(reflect.TypeOf(upolySturmDerivativeRaw{}).Size())
	if e := q.owner.manager.config.Integer.Budget().Reserve(n); e != nil {
		return e
	}
	q.derivativeRetired = &upolySturmDerivativeRaw{value: q.derivative, next: q.derivativeRetired, header: n}
	q.derivative = unumeral.Int{}
	return nil
}

func (q *upolySequenceSturmScope) cleanupDerivativeMultiplier() error {
	z := q.owner.manager.config.Integer
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
