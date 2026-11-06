package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:794-839.

import "errors"

// The source functors declare pa_x, pa_x_y (or pa_x_div_y), pb_y, r_x.
// Their actual owning references are embedded in the charged algebraic core.
type algebraicResultantPolynomialScope struct {
	polynomialCaller  *polynomialAlgebraicEvaluationScope
	refs              [4]PolynomialRef
	variables         [2]polynomialVariableInvocationReceipt
	arithmetic        polynomialArithmeticInvocationReceipt
	toPolynomial      [2]polynomialToPolynomialInvocationReceipt
	resultant         polynomialGCDInvocationReceipt
	conversion        polynomialFactorNumeralVectorReceipt
	constructed       uint8
	active, executing bool
	sourceStop        error
}

// Standalone borrowed cores materialize this member on first use. The owning
// AlgebraicManager already constructed it and its variables in source order.
func (m *AlgebraicRootCore) ensureMultivariate() error {
	if m.multivariate == nil {
		var e error
		m.multivariate, e = NewMultivariatePolynomialManager(MultivariatePolynomialManagerConfig{
			Core: MultivariatePolynomialConfig{
				Integer: m.config.Rationals.Integers(), Rationals: m.config.Rationals,
				Checkpoint: m.config.Checkpoint, Cooperation: m.config.Cooperation,
			},
			Allocator: m.config.Allocator, Factorization: m.config.Factors.config,
		})
		if e != nil {
			return e
		}
	}
	if e := m.multivariate.ready(); e != nil {
		return e
	}
	for m.resultantVariableCount < 2 {
		x, e := m.multivariate.core.monomials.MkVar()
		if e != nil {
			return e
		}
		m.resultantVariables[m.resultantVariableCount] = x
		m.resultantVariableCount++
	}
	return nil
}

func (m *AlgebraicRootCore) addResultPolynomial(a, b *Algebraic, out UPolynomial, subtract bool) error {
	kind := uint8(0)
	if subtract {
		kind = 1
	}
	return m.binaryResultPolynomial(a, b, out, kind)
}
func (m *AlgebraicRootCore) mulResultPolynomial(a, b *Algebraic, out UPolynomial) error {
	return m.binaryResultPolynomial(a, b, out, 2)
}

func (m *AlgebraicRootCore) binaryResultPolynomial(a, b *Algebraic, out UPolynomial, kind uint8) (cause error) {
	la, e := m.location(a)
	if e != nil {
		return e
	}
	lb, e := m.location(b)
	if e != nil {
		return e
	}
	if la == nil || lb == nil || la.kind != 1 || lb.kind != 1 {
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
	ra, e := m.load(la)
	if e != nil {
		return e
	}
	rb, e := m.load(lb)
	if e != nil {
		return e
	}
	da, e := p.descriptor(ra.poly)
	if e != nil {
		return e
	}
	db, e := p.descriptor(rb.poly)
	if e != nil {
		return e
	}
	x, y := m.resultantVariables[0], m.resultantVariables[1]
	pa, e := mm.toPolynomialWithReceipt(ra.size, p.values(&da.state), x, &s.toPolynomial[0])
	if e != nil {
		return e
	}
	if e = s.refs[0].Assign(pa); e != nil {
		return e
	}
	pb, e := mm.toPolynomialWithReceipt(rb.size, p.values(&db.state), y, &s.toPolynomial[1])
	if e != nil {
		return e
	}
	if e = s.refs[2].Assign(pb); e != nil {
		return e
	}
	switch kind {
	case 0:
		e = mm.ComposeXMinusY(pa, y, &s.refs[1])
	case 1:
		e = mm.ComposeXPlusY(pa, y, &s.refs[1])
	case 2:
		var composed Polynomial
		composed, e = mm.ComposeXDivY(pa, y)
		if e == nil {
			e = s.refs[1].Assign(composed)
		}
	}
	if e != nil {
		return e
	}
	if e = mm.resultantWithReceipt(s.refs[1].root.value, pb, y, &s.refs[3].root, &s.resultant); e != nil {
		return e
	}
	return mm.toNumeralVectorIntoSource(s.refs[3].root.value, 0, false, p, &destination.state, &s.conversion)
}

// Finish source reference destructors without acknowledging stopped children.
func (m *AlgebraicRootCore) finishResultantPolynomial() error {
	s := &m.resultant
	s.executing = false
	if m.multivariate == nil || s.constructed == 0 {
		s.active = false
		return nil
	}
	s.executing = true
	defer func() { s.executing = false }()
	c := m.multivariate.core
	if e := c.arithmeticInvocationSourceCause(&s.arithmetic); e != nil {
		s.sourceStop = e
		return e
	}
	for s.constructed > 0 {
		if e := s.refs[s.constructed-1].root.closeValue(c, false); e != nil {
			s.sourceStop = e
			return e
		}
		s.constructed--
	}
	return nil
}

func (m *AlgebraicRootCore) cleanupResultantPolynomial() error {
	s := &m.resultant
	if s.executing {
		return ErrAlgebraicPending
	}
	if !s.active || m.multivariate == nil {
		return nil
	}
	s.executing = true
	defer func() { s.executing = false }()
	mm := m.multivariate
	if e := mm.core.cleanupArithmeticInvocation(&s.arithmetic, false); e != nil {
		return e
	}
	if e := mm.cleanupFactorNumeralVector(&s.conversion, false); e != nil {
		return e
	}
	if e := mm.cleanupGCDInvocation(&s.resultant, false); e != nil {
		return e
	}
	if e := mm.CleanupComposition(); e != nil {
		return e
	}
	for i := 1; i >= 0; i-- {
		if e := mm.cleanupToPolynomialInvocation(&s.toPolynomial[i], false); e != nil {
			return e
		}
	}
	for i := len(s.refs) - 1; i >= 0; i-- {
		if e := s.refs[i].root.close(mm.core); e != nil {
			return e
		}
	}
	if e := mm.cleanupFactorNumeralVector(&s.conversion, true); e != nil {
		return e
	}
	if e := mm.cleanupGCDInvocation(&s.resultant, true); e != nil {
		return e
	}
	for i := 1; i >= 0; i-- {
		if e := mm.cleanupToPolynomialInvocation(&s.toPolynomial[i], true); e != nil {
			return e
		}
	}
	for i := len(s.variables) - 1; i >= 0; i-- {
		if e := mm.core.cleanupVariableInvocation(&s.variables[i], true); e != nil {
			return e
		}
	}
	if e := mm.core.cleanupArithmeticInvocation(&s.arithmetic, true); e != nil {
		return e
	}
	s.active = false
	return nil
}
