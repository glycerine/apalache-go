package polynomial

// Source: src/math/polynomial/polynomial.cpp:2582-2594.

import (
	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Concrete borrowed source numeral array routes. None acquires a second
// owning backing or preflights an unread later coefficient.

type polynomialUnivariateCoefficients struct {
	values  []unumeral.Int
	array   unumeral.IntegerArrayView
	indexed bool
}

func (as polynomialUnivariateCoefficients) reference(m *MultivariatePolynomialCore, k uint32, local *unumeral.Int) (*unumeral.Int, error) {
	if as.indexed {
		return as.array.Reference(k, local)
	}
	if uint64(k) >= uint64(len(as.values)) {
		return nil, arena.ErrBounds
	}
	return &as.values[k], nil
}
func (as polynomialUnivariateCoefficients) store(m *MultivariatePolynomialCore, k uint32, a *unumeral.Int) error {
	if as.indexed {
		return as.array.Store(k, *a)
	}
	return nil // actual slice field already received the source mutation
}
func (m *MultivariatePolynomialCore) mkUnivariateSource(v, n uint32, as polynomialUnivariateCoefficients) (Polynomial, error) {
	return m.mkUnivariateToPolynomial(v, n, as, nil)
}
func (m *MultivariatePolynomialCore) mkUnivariateToPolynomial(v, n uint32, as polynomialUnivariateCoefficients, r *polynomialToPolynomialInvocationReceipt) (Polynomial, error) {
	size, e := m.cheap.ms.size(m)
	if e != nil {
		return Polynomial{}, e
	}
	if e = polynomialUnivariateSourceAssert(2583, "m_cheap_som_buffer.empty()", func() (bool, error) {
		return size == 0, nil
	}); e != nil {
		return Polynomial{}, e
	}
	if r != nil {
		if !r.manager.matchesToPolynomialInvocation(r) || r.manager.core != m || !r.frame.executing || m.callbackDepth != 0 {
			return Polynomial{}, ErrPolynomialOwner
		}
		if e := m.cheap.acquireAppendInvocation(&r.frame.append, size); e != nil {
			return Polynomial{}, e
		}
	}
	k := n + 1
	for k > 0 {
		k--
		var local unumeral.Int
		a, e := as.reference(m, k, &local)
		if e != nil {
			return Polynomial{}, e
		}
		zero, e := m.modular.IsZero(a)
		if e != nil {
			return Polynomial{}, e
		}
		if zero {
			e = m.deleteNumeral(a)
			if w := as.store(m, k, a); w != nil {
				return Polynomial{}, polynomialNewtonJoin(e, w)
			}
			if e != nil {
				return Polynomial{}, e
			}
			continue
		}
		var term Monomial
		if r == nil {
			term, e = m.generatedVariable(v, k)
		} else {
			term, e = m.generatedVariableToPolynomial(v, k, r)
		}
		if e != nil {
			return Polynomial{}, e
		}
		if r == nil {
			e = m.cheap.add(a, term, true)
		} else {
			e = m.cheap.addWithAppendReceipt(a, term, true, &r.frame.append)
		}
		if w := as.store(m, k, a); w != nil {
			return Polynomial{}, polynomialNewtonJoin(e, w)
		}
		if e != nil {
			return Polynomial{}, e
		}
	}
	if r != nil {
		return m.cheap.mkWithAppendReceipt(false, &r.frame.append, &r.frame.factory)
	}
	return m.cheap.mk(false)
}
