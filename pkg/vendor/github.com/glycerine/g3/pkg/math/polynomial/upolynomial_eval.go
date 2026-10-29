package polynomial

// Source: src/math/polynomial/upolynomial.cpp:1734-1820.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *UPolynomialManager) sign(a *unumeral.Int) (int, error) {
	zero, e := m.modular.IsZero(a)
	if e != nil {
		return 0, e
	}
	if zero {
		return 0, nil
	}
	pos, e := m.modular.IsPos(a)
	if e != nil {
		return 0, e
	}
	if pos {
		return 1, nil
	}
	return -1, nil
}
func (m *UPolynomialManager) signAt(v UPolynomialView, i uint32) (int, error) {
	a, e := m.read(v, i)
	if e != nil {
		return 0, e
	}
	return m.sign(a)
}
func (m *UPolynomialManager) EvalSignInt(v UPolynomialView, b *unumeral.Int) (sign int, cause error) {
	return m.evalSignIntegerWithReceipt(v, b, nil)
}
func (m *UPolynomialManager) evalSignIntegerWithReceipt(v UPolynomialView, b *unumeral.Int, receipt *upolyIntegerEvaluationInvocationReceipt) (sign int, cause error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	if v.count == 0 {
		return 0, nil
	}
	if v.count == 1 {
		return m.signAt(v, 0)
	}
	if b == nil {
		return 0, ErrUPolynomialOwner
	}
	q, e := m.beginIntegerEvaluation()
	if e != nil {
		return 0, e
	}
	if receipt != nil {
		*receipt = upolyIntegerEvaluationInvocationReceipt{owner: m, frame: q, invocation: q.invocation}
	}
	q.target = v
	context := q.context()
	z := m.config.Integer
	prior := integerEvaluationCaller(z)
	setIntegerEvaluationCaller(z, m)
	defer func() {
		q.executing = false
		cause = q.finish(cause)
		if receipt != nil {
			receipt.history = q.history
		}
		setIntegerEvaluationCaller(z, prior)
	}()
	r := &q.values[0]
	a, e := m.read(v, v.count-1)
	if e != nil {
		return 0, e
	}
	if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Copy(r, a); e != nil {
		return 0, e
	}
	for i := v.count - 1; i > 0; {
		i--
		a, e := m.read(v, i)
		if e != nil {
			return 0, e
		}
		zero, e := m.modular.IsZero(a)
		if e != nil {
			return 0, e
		}
		if zero {
			e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Mul(r, b, r)
		} else {
			e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).AddMulTracked(a, r, b, r, &q.addMul)
		}
		if e != nil {
			return 0, e
		}
	}
	return m.sign(r)
}
func (m *UPolynomialManager) EvalSignRational(v UPolynomialView, b *unumeral.Rational) (sign int, cause error) {
	return m.evalSignRationalWithReceipt(v, b, nil)
}
func (m *UPolynomialManager) evalSignRationalWithReceipt(v UPolynomialView, b *unumeral.Rational, receipt *upolyRationalEvaluationInvocationReceipt) (sign int, cause error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	if v.count == 0 {
		return 0, nil
	}
	if v.count == 1 {
		return m.signAt(v, 0)
	}
	if b == nil {
		return 0, ErrUPolynomialOwner
	}
	q, e := m.beginRationalEvaluation()
	if e != nil {
		return 0, e
	}
	if receipt != nil {
		*receipt = upolyRationalEvaluationInvocationReceipt{owner: m, frame: q, invocation: q.invocation}
	}
	q.target = v
	context := q.context()
	z := m.config.Integer
	prior := rationalEvaluationCaller(z)
	setRationalEvaluationCaller(z, m)
	defer func() {
		q.executing = false
		cause = q.finish(cause)
		if receipt != nil {
			receipt.history = q.history
		}
		setRationalEvaluationCaller(z, prior)
	}()
	di, r, ak := &q.values[0], &q.values[1], &q.values[2]
	if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Copy(di, &(*b.BorrowDenominator())); e != nil {
		return 0, e
	}
	// Actual r and ak constructors occur only after the denominator SET returns.
	q.constructed = 3
	q.sourceRemaining = 3
	a, e := m.read(v, v.count-1)
	if e != nil {
		return 0, e
	}
	if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Copy(r, a); e != nil {
		return 0, e
	}
	for i := v.count - 1; i > 0; {
		i--
		a, e := m.read(v, i)
		if e != nil {
			return 0, e
		}
		zero, e := m.modular.IsZero(a)
		if e != nil {
			return 0, e
		}
		if zero {
			e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Mul(r, &(*b.BorrowNumerator()), r)
		} else {
			e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Mul(a, di, ak)
			if e == nil {
				e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).AddMulTracked(ak, r, &(*b.BorrowNumerator()), r, &q.addMul)
			}
		}
		if e != nil {
			return 0, e
		}
		if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Mul(di, &(*b.BorrowDenominator()), di); e != nil {
			return 0, e
		}
	}
	return m.sign(r)
}
func (m *UPolynomialManager) EvalSignBinary(v UPolynomialView, b *unumeral.BinaryRational) (int, error) {
	return m.evalSignBinaryWithNegScope(v, b, nil)
}
func (m *UPolynomialManager) evalSignBinaryWithNegScope(v UPolynomialView, b *unumeral.BinaryRational, parent *algebraicNegScope) (value int, cause error) {
	return m.evalSignBinaryWithNegScopeAndReceipt(v, b, parent, nil)
}
func (m *UPolynomialManager) evalSignBinaryWithNegScopeAndReceipt(v UPolynomialView, b *unumeral.BinaryRational, parent *algebraicNegScope, receipt *upolyBinaryEvaluationInvocationReceipt) (value int, cause error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	if v.count == 0 {
		return 0, nil
	}
	if v.count == 1 {
		return m.signAt(v, 0)
	}
	if b == nil {
		return 0, ErrUPolynomialOwner
	}
	k := b.K()
	ki := k
	var values *[2]unumeral.Int
	context := integerAllocationInvocationContext{}
	var child *unumeral.IntegerAddMulInvocationReceipt
	if parent == nil {
		q, e := m.beginBinaryEvaluation()
		if e != nil {
			return 0, e
		}
		// Stamp only this accepted entry. Its diagnostic history can be
		// materialized by the body or destructor, so attach that same
		// frame's history after source finalization has returned.
		if receipt != nil {
			*receipt = upolyBinaryEvaluationInvocationReceipt{m, q, q.invocation, q.history}
			defer func() { receipt.history = q.history }()
		}
		q.target = v
		values = &q.values
		context = q.context()
		child = &q.addMul
		z := m.config.Integer
		old := binaryEvaluationCaller(z)
		setBinaryEvaluationCaller(z, m)
		defer func() { q.executing = false; cause = q.finish(cause); setBinaryEvaluationCaller(z, old) }()
	} else {
		if parent.context(false).algebraicNegFrame(m.config.Integer) == nil || parent.owner.config.Polynomials != m {
			return 0, ErrAlgebraicOwner
		}
		if parent.eval.active {
			return 0, ErrAlgebraicPending
		}
		evalInvocation := parent.eval.invocation + 1
		if evalInvocation == 0 {
			return 0, ErrAlgebraicPending
		}
		parent.eval = algebraicNegBinaryScope{invocation: evalInvocation, constructed: 2, sourceRemaining: 2, active: true, executing: true}
		values = &parent.eval.values
		context = parent.context(true)
		child = &parent.eval.addMul
		defer func() {
			q := &parent.eval
			q.executing = false
			q.bodyReturned = true
			if q.bodyCause == nil {
				q.bodyCause = cause
			}
			cause = parent.finishBinarySource(cause)
		}()
	}
	r, ak := &values[0], &values[1]
	a, e := m.read(v, v.count-1)
	if e != nil {
		return 0, e
	}
	if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Copy(r, a); e != nil {
		return 0, e
	}
	for i := v.count - 1; i > 0; {
		i--
		a, e := m.read(v, i)
		if e != nil {
			return 0, e
		}
		zero, e := m.modular.IsZero(a)
		if e != nil {
			return 0, e
		}
		if zero {
			e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Mul(r, &(*b.BorrowNumerator()), r)
		} else {
			e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Mul2k(a, ki, ak)
			if e == nil {
				e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).AddMulTracked(ak, r, &(*b.BorrowNumerator()), r, child)
			}
		}
		if e != nil {
			return 0, e
		}
		ki += k
	}
	return m.sign(r)
}
func (m *UPolynomialManager) EvalSignZero(v UPolynomialView) (int, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	if v.count == 0 {
		return 0, nil
	}
	return m.signAt(v, 0)
}
func (m *UPolynomialManager) EvalSignPlusInfinity(v UPolynomialView) (int, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	if v.count == 0 {
		return 0, nil
	}
	return m.signAt(v, v.count-1)
}
func (m *UPolynomialManager) EvalSignMinusInfinity(v UPolynomialView) (int, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	if v.count == 0 {
		return 0, nil
	}
	sign, e := m.signAt(v, v.count-1)
	if (v.count-1)%2 != 0 {
		sign = -sign
	}
	return sign, e
}
