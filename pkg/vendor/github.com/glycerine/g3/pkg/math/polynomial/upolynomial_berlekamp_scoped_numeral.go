package polynomial

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// The actual scoped_numerals at factorization.cpp:110/:160 and
// upolynomial.cpp:460. Source
// destruction and separate owning cleanup retain independent completion.
type upolyBerlekampScopedNumeral struct {
	value                           unumeral.Int
	declared, sourceEnded, ownEnded bool
	cause                           error
}

func (q *upolyBerlekampMatrix) finishScopedNumeral(local *upolyBerlekampScopedNumeral, body error) error {
	if !local.declared || local.sourceEnded {
		return body
	}
	if e := q.numeric.sourceCause(); e != nil {
		if local.cause == nil {
			local.cause = e
		}
		if q.closeCause == nil {
			q.closeCause = e
		}
		q.m.pendingBerlekamp = q
		return errors.Join(body, e)
	}
	if local.cause != nil {
		return errors.Join(body, local.cause)
	}
	q.destroying = true
	defer func() { q.destroying = false }()
	var borrow upolyFactorTraceBorrow
	e := borrow.bindManagers([3]*UPolynomialManager{q.m}, q.m.modular.BorrowTraceTags())
	if e == nil {
		defer borrow.unbind()
		_, e = upolynomialSourceNumeralDel(q.m.config.Integer, &local.value)
	}
	if e == nil {
		local.sourceEnded = true
		return body
	}
	local.cause = e
	if q.closeCause == nil {
		q.closeCause = e
	}
	if q.m.pendingBerlekamp == nil || q.m.pendingBerlekamp == q {
		q.m.pendingBerlekamp = q
	}
	return errors.Join(body, e)
}
