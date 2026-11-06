package polynomial

// Source: src/math/polynomial/polynomial.cpp:2558-2633.

import (
	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *MultivariatePolynomialCore) rational2Numeral(as []unumeral.Rational, n uint32) error {
	if e := m.cellSourceAssert(2558, "m_rat2numeral.empty()", func() (bool, error) {
		n, e := m.rationalTerms.size(m)
		return n == 0, e
	}); e != nil {
		return e
	}
	var e error
	for i := uint32(0); i < n; i++ {
		if e = m.cellSourceAssert(2560, "as[i].is_int()", func() (bool, error) {
			if uint64(i) >= uint64(len(as)) {
				return false, arena.ErrBounds
			}
			return unumeral.IsIntegerRational(&as[i]), nil
		}); e != nil {
			return e
		}
		if e = m.rationalTerms.appendCoefficient(m, unumeral.Int{}); e != nil {
			return e
		}
		pos, e := m.rationalTerms.size(m)
		if e != nil {
			return e
		}
		a, e := m.rationalTerms.coefficient(m, pos-1)
		if e != nil {
			return e
		}
		if uint64(i) >= uint64(len(as)) {
			return arena.ErrBounds
		}
		e = m.modular.Copy(&a, &(*as[i].BorrowNumerator()))
		if w := m.rationalTerms.putCoefficient(m, pos-1, a); w != nil {
			return w
		}
		if e != nil {
			return e
		}
	}
	return nil
}
func (m *MultivariatePolynomialCore) resetRationalTerms() error {
	if polynomialSourceDebug {
		for i := uint32(0); ; i++ {
			n, e := m.rationalTerms.size(m)
			if e != nil {
				return e
			}
			if i >= n {
				break
			}
			if e = m.cellSourceAssert(2569, "m_manager.is_zero(m_rat2numeral[i])", func() (bool, error) {
				a, e := m.rationalTerms.coefficient(m, i)
				if e != nil {
					return false, e
				}
				return m.modular.IsZero(&a)
			}); e != nil {
				return e
			}
		}
	}
	return m.rationalTerms.reset(m)
}
func (m *MultivariatePolynomialCore) mkTermsIndexed(as *polynomialNumerals, ms monomialArrayView) (Polynomial, error) {
	if e := m.som.reset(); e != nil {
		return Polynomial{}, e
	}
	n, e := as.size(m)
	if e != nil {
		return Polynomial{}, e
	}
	return m.mkTermsSource(n, polynomialUnivariateCoefficients{values: as.values}, ms)
}
func (m *MultivariatePolynomialCore) mkTermsSource(n uint32, as polynomialUnivariateCoefficients, ms monomialArrayView) (Polynomial, error) {
	return m.mkTermsSourceLinear(n, as, ms, false)
}
func (m *MultivariatePolynomialCore) mkTermsSourceLinear(n uint32, as polynomialUnivariateCoefficients, ms monomialArrayView, linear bool) (Polynomial, error) {
	return m.mkTermsSourceReceipts(n, as, ms, linear, nil, nil)
}
func (m *MultivariatePolynomialCore) mkTermsSourceReceipts(n uint32, as polynomialUnivariateCoefficients, ms monomialArrayView, linear bool, appendReceipt *polynomialCheapAppendInvocationReceipt, factoryReceipt *polynomialFactoryInvocationReceipt) (Polynomial, error) {
	for i := uint32(0); i < n; i++ {
		var local unumeral.Int
		a, e := as.reference(m, i, &local)
		if e != nil {
			return Polynomial{}, e
		}
		term, e := ms.at(i)
		if e != nil {
			return Polynomial{}, e
		}
		consumed := false
		e = m.som.addConsumedReceipts(a, term, false, &consumed, nil, appendReceipt)
		if linear && consumed && uint64(i) < uint64(len(m.linearGenerated)) {
			m.linearGenerated[i] = false
		}
		if e != nil {
			return Polynomial{}, e
		}
	}
	return m.som.mkWithAppendReceipt(false, appendReceipt, factoryReceipt)
}
func (m *MultivariatePolynomialCore) MkRationalTerms(as []unumeral.Rational, ms []Monomial) (Polynomial, error) {
	if e := m.check(); e != nil {
		return Polynomial{}, e
	}
	if m.overloadResult != (Polynomial{}) {
		return Polynomial{}, ErrPolynomialPending
	}
	if uint64(len(as)) > uint64(^uint32(0)) {
		return Polynomial{}, arena.ErrBounds
	}
	if e := m.rational2Numeral(as, uint32(len(as))); e != nil {
		return Polynomial{}, e
	}
	p, e := m.mkTermsIndexed(&m.rationalTerms, monomialArrayView{values: ms, count: uint32(len(ms))})
	if e != nil {
		return Polynomial{}, e
	}
	m.overloadResult = p
	if e = m.resetRationalTerms(); e != nil {
		return Polynomial{}, e
	}
	m.overloadResult = Polynomial{}
	return p, nil
}
func (m *MultivariatePolynomialCore) MkRationalUnivariate(v, n uint32, as []unumeral.Rational) (Polynomial, error) {
	if e := m.check(); e != nil {
		return Polynomial{}, e
	}
	if m.overloadResult != (Polynomial{}) {
		return Polynomial{}, ErrPolynomialPending
	}
	if polynomialSourceDebug && v >= m.monomials.nextVar {
		return Polynomial{}, ErrPolynomialAssertion
	}
	if e := m.rational2Numeral(as, n+1); e != nil {
		return Polynomial{}, e
	}
	p, e := m.mkUnivariateSource(v, n, polynomialUnivariateCoefficients{values: m.rationalTerms.values})
	if e != nil {
		return Polynomial{}, e
	}
	m.overloadResult = p
	if e = m.resetRationalTerms(); e != nil {
		return Polynomial{}, e
	}
	m.overloadResult = Polynomial{}
	return p, nil
}
func (m *MultivariatePolynomialCore) MkLinearTake(as []unumeral.Int, xs []uint32, c *unumeral.Int) (Polynomial, error) {
	if e := m.check(); e != nil {
		return Polynomial{}, e
	}
	if m.overloadResult != (Polynomial{}) {
		return Polynomial{}, ErrPolynomialPending
	}
	if e := m.linearConstructorEmptyAssertions(); e != nil {
		return Polynomial{}, e
	}
	return m.mkLinearInput(as, uint32(len(as)), xs, c)
}
func (m *MultivariatePolynomialCore) mkLinearInput(as []unumeral.Int, count uint32, xs []uint32, c *unumeral.Int) (Polynomial, error) {
	var e error
	for i := uint32(0); i < count; i++ {
		if uint64(i) >= uint64(len(as)) {
			return Polynomial{}, arena.ErrBounds
		}
		original := &as[i]
		zero, e := m.modular.IsZero(original)
		if e != nil {
			return Polynomial{}, e
		}
		if zero {
			continue
		}
		if e = m.linearTerms.appendCoefficient(m, unumeral.Int{}); e != nil {
			return Polynomial{}, e
		}
		n, e := m.linearTerms.size(m)
		if e != nil {
			return Polynomial{}, e
		}
		a, e := m.linearTerms.coefficient(m, n-1)
		if e != nil {
			return Polynomial{}, e
		}
		if e = m.modular.Swap(&a, original); e != nil {
			return Polynomial{}, e
		}
		if e = m.linearTerms.putCoefficient(m, n-1, a); e != nil {
			return Polynomial{}, e
		}
		if uint64(i) >= uint64(len(xs)) {
			return Polynomial{}, arena.ErrBounds
		}
		term, e := m.generatedVariable(xs[i], 1)
		if e != nil {
			return Polynomial{}, e
		}
		index := uint32(len(m.linearMonomials))
		m.prepareLinearGenerated(index)
		m.linearMonomials = append(m.linearMonomials, term)
		m.linearGenerated[index] = m.generatedActive
		m.transferGeneratedMonomial()
	}
	zero, e := m.modular.IsZero(c)
	if e != nil {
		return Polynomial{}, e
	}
	if !zero {
		if e = m.linearTerms.appendCoefficient(m, unumeral.Int{}); e != nil {
			return Polynomial{}, e
		}
		n, e := m.linearTerms.size(m)
		if e != nil {
			return Polynomial{}, e
		}
		a, e := m.linearTerms.coefficient(m, n-1)
		if e != nil {
			return Polynomial{}, e
		}
		if e = m.modular.Swap(&a, c); e != nil {
			return Polynomial{}, e
		}
		if e = m.linearTerms.putCoefficient(m, n-1, a); e != nil {
			return Polynomial{}, e
		}
		u, e := m.monomials.Unit()
		if e != nil {
			return Polynomial{}, e
		}
		m.linearMonomials = append(m.linearMonomials, u)
	}
	if e := m.som.reset(); e != nil {
		return Polynomial{}, e
	}
	n, e := m.linearTerms.size(m)
	if e != nil {
		return Polynomial{}, e
	}
	p, e := m.mkTermsSourceLinear(n, polynomialUnivariateCoefficients{values: m.linearTerms.values}, monomialArrayView{values: m.linearMonomials, count: uint32(len(m.linearMonomials))}, true)
	if e != nil {
		return Polynomial{}, e
	}
	m.overloadResult = p
	if e = m.linearTerms.reset(m); e != nil {
		return Polynomial{}, e
	}
	clear(m.linearMonomials)
	m.linearMonomials = m.linearMonomials[:0]
	m.overloadResult = Polynomial{}
	return p, nil
}

func (m *MultivariatePolynomialCore) MkRationalLinear(as []unumeral.Rational, xs []uint32, c *unumeral.Rational) (Polynomial, error) {
	if e := m.check(); e != nil {
		return Polynomial{}, e
	}
	if c == nil {
		return Polynomial{}, arena.ErrBounds
	}
	if e := m.cellSourceAssert(2626, "c.is_int()", func() (bool, error) { return unumeral.IsIntegerRational(c), nil }); e != nil {
		return Polynomial{}, e
	}
	if m.overloadResult != (Polynomial{}) || m.linearConstantActive {
		return Polynomial{}, ErrPolynomialPending
	}
	if uint64(len(as)) > uint64(^uint32(0)) {
		return Polynomial{}, arena.ErrBounds
	}
	if e := m.rational2Numeral(as, uint32(len(as))); e != nil {
		return Polynomial{}, e
	}
	m.linearConstant = unumeral.Int{}
	m.linearConstantActive = true
	if e := m.modular.Copy(&m.linearConstant, &(*c.BorrowNumerator())); e != nil {
		return Polynomial{}, e
	}
	if e := m.linearConstructorEmptyAssertions(); e != nil {
		return Polynomial{}, e
	}
	p, e := m.mkLinearInput(m.rationalTerms.values, uint32(len(as)), xs, &m.linearConstant)
	if e != nil {
		return Polynomial{}, e
	}
	m.overloadResult = p
	if e = m.cellSourceAssert(2631, "m_manager.is_zero(tmp_c)", func() (bool, error) { return m.modular.IsZero(&m.linearConstant) }); e != nil {
		return Polynomial{}, e
	}
	if e = m.resetRationalTerms(); e != nil {
		return Polynomial{}, e
	}
	// Bare source numeral has no manager destructor. Completed source swaps
	// leave zero here; a failed body retains actual fields for owning abort.
	m.linearConstantActive = false
	m.overloadResult = Polynomial{}
	return p, nil
}
