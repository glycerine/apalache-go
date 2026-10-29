package polynomial

// Source: src/math/polynomial/polynomial.cpp:5817-5895; src/util/obj_ref.h:31-112.

func (m *MultivariatePolynomialCore) pscClassicChildSourceCause() error {
	s := &m.pscClassic
	switch s.phase {
	case 1:
		return m.coefficientInvocationSourceCause(&s.coefficient)
	case 2:
		return m.powerInvocationSourceCause(&s.power)
	case 3:
		return m.negInvocationSourceCause(&s.neg)
	case 4:
		return m.pseudoInvocationSourceCause(&s.pseudo)
	case 5:
		return m.arithmeticInvocationSourceCause(&s.arithmetic)
	case 6:
		return m.exactDivInvocationSourceCause(&s.division)
	}
	return nil
}
func (m *MultivariatePolynomialCore) pscClassicChildren(abort bool) error {
	s := &m.pscClassic
	if e := m.cleanupExactDivInvocation(&s.division, abort); e != nil {
		return e
	}
	if e := m.cleanupArithmeticInvocation(&s.arithmetic, abort); e != nil {
		return e
	}
	if e := m.cleanupPseudoInvocation(&s.pseudo, abort); e != nil {
		return e
	}
	if e := m.cleanupNegInvocation(&s.neg, abort); e != nil {
		return e
	}
	if e := m.cleanupPowerInvocation(&s.power, abort); e != nil {
		return e
	}
	if e := m.cleanupCoefficientInvocation(&s.coefficient, abort); e != nil {
		return e
	}
	if abort {
		if e := m.cleanupFactoryInvocation(&s.factory, true); e != nil {
			return e
		}
		if e := m.cheap.cleanupAppendInvocation(&s.append, true); e != nil {
			return e
		}
		return nil
	}
	return nil
}
func (m *MultivariatePolynomialCore) pscClassicPending() bool {
	s := &m.pscClassic
	for i := range s.refs {
		r := &s.refs[i].root
		if r.assigning || r.incoming != (Polynomial{}) {
			return true
		}
	}
	return m.exactDivInvocationPending(&s.division) || m.matchesArithmeticInvocation(&s.arithmetic) || m.matchesPseudoInvocation(&s.pseudo) || m.matchesNegInvocation(&s.neg) || m.matchesPowerInvocation(&s.power) || m.matchesCoefficientInvocation(&s.coefficient) || m.matchesFactoryInvocation(&s.factory)
}
func (m *MultivariatePolynomialCore) finishPSCClassicPhase(owning, abort bool) (err error) {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	s := &m.pscClassic
	if s.executing || s.finishing || m.callbackDepth != 0 {
		return ErrPolynomialPending
	}
	if !owning && s.sourceCause != nil {
		return s.sourceCause
	}
	if !s.active {
		return nil
	}
	s.finishing = true
	defer func() {
		s.finishing = false
		if err != nil {
			if owning {
				if s.cleanupCause == nil {
					s.cleanupCause = err
				}
			} else if s.sourceCause == nil {
				s.sourceCause = err
			}
		}
	}()
	if !owning {
		if e := m.pscClassicChildSourceCause(); e != nil {
			return e
		}
	} else if e := m.pscClassicChildren(false); e != nil {
		return e
	}
	for s.cursor < s.constructed {
		if e := s.refs[s.constructed-s.cursor-1].root.closeValue(m, owning); e != nil {
			return e
		}
		s.cursor++
	}
	if abort {
		for i := int(s.constructed) - 1; i >= 0; i-- {
			if e := s.refs[i].root.abort(m); e != nil {
				return e
			}
		}
		if e := m.pscClassicChildren(true); e != nil {
			return e
		}
		if !m.pscClassicPending() {
			s.active = false
		}
	}
	return nil
}
