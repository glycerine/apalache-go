package polynomial

// Source: algebraic_numbers.cpp:1876-1903,2945-2951,2959-2965,2973-2975.

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

// Equal follows source eq(numeral, numeral), including any refinement performed
// by comparison. It does not substitute encoded-cell identity for value equality.
func (m *AlgebraicRootCore) Equal(a, b *Algebraic) (bool, error) {
	order, err := m.Compare(a, b)
	return order == 0 && err == nil, err
}

// Less follows source lt(numeral, numeral).
func (m *AlgebraicRootCore) Less(a, b *Algebraic) (bool, error) {
	order, err := m.Compare(a, b)
	return order < 0 && err == nil, err
}

// EqualRational preserves the source basic-value equality shortcut. Comparing
// cross-products here would change equality for raw, unnormalized rationals.
func (m *AlgebraicRootCore) EqualRational(a *Algebraic, b *unumeral.Rational) (bool, error) {
	l, e := m.location(a)
	if e != nil {
		return false, e
	}
	if l == nil {
		return m.config.Rationals.Equal(&m.zero, b)
	}
	r, e := m.load(l)
	if e != nil {
		return false, e
	}
	if l.kind == 0 {
		return m.config.Rationals.Equal(&r.value, b)
	}
	order, e := m.compareRootRational(&r, b)
	return order == 0 && e == nil, e
}

// LessRational follows imp::lt(numeral, mpq).
func (m *AlgebraicRootCore) LessRational(a *Algebraic, b *unumeral.Rational) (bool, error) {
	l, e := m.location(a)
	if e != nil {
		return false, e
	}
	if l == nil {
		order, e := m.config.Rationals.Compare(&m.zero, b)
		return order < 0 && e == nil, e
	}
	r, e := m.load(l)
	if e != nil {
		return false, e
	}
	if l.kind == 0 {
		order, e := m.config.Rationals.Compare(&r.value, b)
		return order < 0 && e == nil, e
	}
	order, e := m.compareRootRational(&r, b)
	return order < 0 && e == nil, e
}

// GreaterRational preserves qm().gt's reversed rational less-than call.
func (m *AlgebraicRootCore) GreaterRational(a *Algebraic, b *unumeral.Rational) (bool, error) {
	l, e := m.location(a)
	if e != nil {
		return false, e
	}
	if l == nil {
		order, e := m.config.Rationals.Compare(b, &m.zero)
		return order < 0 && e == nil, e
	}
	r, e := m.load(l)
	if e != nil {
		return false, e
	}
	if l.kind == 0 {
		order, e := m.config.Rationals.Compare(b, &r.value)
		return order < 0 && e == nil, e
	}
	order, e := m.compareRootRational(&r, b)
	return order > 0 && e == nil, e
}
