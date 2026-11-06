package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:1164-1183,1227-1244.

import "errors"

// unaryResultPolynomial preserves the source four owning references. Root
// extraction eliminates y from x^k-y,p(y); powering uses x-y^k,p(y).
func (m *AlgebraicRootCore) unaryResultPolynomial(a *Algebraic, k uint32, out UPolynomial, root bool) (cause error) {
	l, e := m.location(a)
	if e != nil {
		return e
	}
	if l == nil || l.kind != 1 {
		return ErrAlgebraicOwner
	}
	p := m.config.Polynomials
	destination, e := p.descriptor(out)
	if e != nil {
		return e
	}
	s := &m.resultant
	if s.active {
		return ErrAlgebraicPending
	}
	*s = algebraicResultantPolynomialScope{polynomialCaller: m.polynomialCaller, active: true, executing: true}
	defer func() {
		cause = errors.Join(cause, m.finishResultantPolynomial())
		if cause == nil {
			s.active = false
		}
	}()
	if e = m.ensureMultivariate(); e != nil {
		return e
	}
	mm := m.multivariate
	for i := range s.refs {
		s.refs[i] = PolynomialRef{owner: mm.core}
		s.refs[i].self = &s.refs[i]
		s.constructed++
	}
	r, e := m.load(l)
	if e != nil {
		return e
	}
	d, e := p.descriptor(r.poly)
	if e != nil {
		return e
	}
	x, y := m.resultantVariables[0], m.resultantVariables[1]
	py, e := mm.toPolynomialWithReceipt(r.size, p.values(&d.state), y, &s.toPolynomial[0])
	if e != nil {
		return e
	}
	if e = s.refs[0].Assign(py); e != nil {
		return e
	}
	// Source declaration order differs: root declares xk_y then y; power
	// declares x then x_yk. Construct the actual variables in source order.
	firstVar, firstPower, firstRef := x, uint32(1), 1
	secondVar, secondPower, secondRef := y, k, 2
	if root {
		firstVar, firstPower, firstRef = y, 1, 2
		secondVar, secondPower, secondRef = x, k, 1
	}
	first, e := mm.core.mkVariableWithReceipt(firstVar, firstPower, &s.variables[0])
	if e != nil {
		return e
	}
	if e = s.refs[firstRef].Assign(first); e != nil {
		return e
	}
	second, e := mm.core.mkVariableWithReceipt(secondVar, secondPower, &s.variables[1])
	if e != nil {
		return e
	}
	if e = s.refs[secondRef].Assign(second); e != nil {
		return e
	}
	left, right, target := first, second, 2
	if root {
		left, right, target = second, first, 1
	}
	difference, e := mm.core.subWithReceipt(left, right, &s.arithmetic)
	if e != nil {
		return e
	}
	if e = s.refs[target].Assign(difference); e != nil {
		return e
	}
	if e = mm.resultantWithReceipt(s.refs[target].root.value, py, y, &s.refs[3].root, &s.resultant); e != nil {
		return e
	}
	return mm.toNumeralVectorIntoSource(s.refs[3].root.value, 0, false, p, &destination.state, &s.conversion)
}
