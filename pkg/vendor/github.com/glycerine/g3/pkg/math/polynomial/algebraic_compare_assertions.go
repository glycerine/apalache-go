package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:1710,1761-1762; algebraic_numbers.h:373.

func (m *AlgebraicRootCore) assertComparisonNonbasic(a *Algebraic, line int32, expression string) error {
	_, enabled, e := algebraicAssertionSwitch()
	if e != nil || !enabled {
		return e
	}
	l, e := m.location(a)
	if e != nil {
		return e
	}
	if l == nil || l.kind == 0 {
		return m.predicateAssertionFailure(line, expression)
	}
	return nil
}

func (m *AlgebraicRootCore) assertComparisonPair(a, b *Algebraic) error {
	_, enabled, e := algebraicAssertionSwitch()
	if e != nil || !enabled {
		return e
	}
	l, e := m.location(a)
	if e != nil {
		return e
	}
	valid := l != nil && l.kind != 0
	if valid {
		l, e = m.location(b)
		if e != nil {
			return e
		}
		valid = l != nil && l.kind != 0
	}
	if !valid {
		return m.predicateAssertionFailure(1710, "!a.is_basic() && !b.is_basic()")
	}
	return nil
}
