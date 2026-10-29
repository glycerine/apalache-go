package polynomial

// Source: src/math/polynomial/polynomial.cpp:2582-2594,2665-2778,5940-5951.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *MultivariatePolynomialCore) checkpoint() error {
	if f := m.factorsOwner; f != nil && f.executing {
		f.executing = false
		defer func() { f.executing = true }()
	}
	if s := m.compositionOwner; s != nil && s.executing {
		s.executing = false
		defer func() { s.executing = true }()
	}

	// These hooks are external source resource/cooperation boundaries. Their
	// primitive constructions belong to the caller, not an executing PSC frame.
	m.callbackDepth++
	defer func() { m.callbackDepth-- }()

	if !m.config.Checkpoint() {
		return ErrUPolynomialCanceled
	}
	return m.config.Cooperation()
}
func (m *MultivariatePolynomialCore) AddMul(a1 *unumeral.Int, t1 Monomial, p1 Polynomial, a2 *unumeral.Int, t2 Monomial, p2 Polynomial) (Polynomial, error) {
	return m.addMulWithReceipt(a1, t1, p1, a2, t2, p2, nil)
}
func (m *MultivariatePolynomialCore) addMulWithReceipt(a1 *unumeral.Int, t1 Monomial, p1 Polynomial, a2 *unumeral.Int, t2 Monomial, p2 Polynomial, receipt *polynomialArithmeticInvocationReceipt) (Polynomial, error) {
	if e := m.check(); e != nil {
		return Polynomial{}, e
	}
	if e := m.som.reset(); e != nil {
		return Polynomial{}, e
	}
	if receipt != nil {
		if e := m.som.acquireArithmeticInvocation(receipt); e != nil {
			return Polynomial{}, e
		}
	}
	if e := (m.som).addMulWithArithmeticReceipt(a1, t1, p1, receipt); e != nil {
		return Polynomial{}, e
	}
	if e := (m.som).addMulWithArithmeticReceipt(a2, t2, p2, receipt); e != nil {
		return Polynomial{}, e
	}
	return m.som.mkWithArithmeticReceipt(false, receipt)
}
func (m *MultivariatePolynomialCore) Add(p, q Polynomial) (Polynomial, error) {
	return m.addWithReceipt(p, q, nil)
}
func (m *MultivariatePolynomialCore) addWithReceipt(p, q Polynomial, receipt *polynomialArithmeticInvocationReceipt) (Polynomial, error) {
	if e := m.check(); e != nil {
		return Polynomial{}, e
	}
	one := unumeral.NewInt(1)
	u, e := m.monomials.Unit()
	if e != nil {
		return Polynomial{}, e
	}
	return m.addMulWithReceipt(&one, u, p, &one, u, q, receipt)
}
func (m *MultivariatePolynomialCore) Sub(p, q Polynomial) (Polynomial, error) {
	return m.subWithReceipt(p, q, nil)
}
func (m *MultivariatePolynomialCore) subWithReceipt(p, q Polynomial, receipt *polynomialArithmeticInvocationReceipt) (Polynomial, error) {
	if e := m.check(); e != nil {
		return Polynomial{}, e
	}
	one := unumeral.NewInt(1)
	if e := m.modular.SetInt64(&m.minusOne, -1); e != nil {
		return Polynomial{}, e
	}
	u, e := m.monomials.Unit()
	if e != nil {
		return Polynomial{}, e
	}
	return m.addMulWithReceipt(&one, u, p, &m.minusOne, u, q, receipt)
}
func (m *MultivariatePolynomialCore) MulAdd(p, q Polynomial, a *unumeral.Int) (Polynomial, error) {
	return m.mulAddWithReceipt(p, q, a, nil)
}
func (m *MultivariatePolynomialCore) mulAddWithReceipt(p, q Polynomial, a *unumeral.Int, receipt *polynomialArithmeticInvocationReceipt) (Polynomial, error) {
	if e := m.check(); e != nil {
		return Polynomial{}, e
	}
	np, e := m.readWord(p, 8)
	if e != nil {
		return Polynomial{}, e
	}
	if np == 0 {
		return m.mkConstantTakeCopyWithArithmeticReceipt(a, receipt)
	}
	nq, e := m.readWord(q, 8)
	if e != nil {
		return Polynomial{}, e
	}
	if np == 0 || nq == 0 {
		return m.mkConstantTakeCopyWithArithmeticReceipt(a, receipt)
	}
	if e = m.som.reset(); e != nil {
		return Polynomial{}, e
	}
	if receipt != nil {
		if e = m.som.acquireArithmeticInvocation(receipt); e != nil {
			return Polynomial{}, e
		}
	}
	for i := uint32(0); i < np; i++ {
		if e = m.checkpoint(); e != nil {
			return Polynomial{}, e
		}
		coef, e := m.coefficientCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		term, e := m.monomialCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		if e = (m.som).addMulWithArithmeticReceipt(&coef, term, q, receipt); e != nil {
			return Polynomial{}, e
		}
	}
	u, e := m.monomials.Unit()
	if e != nil {
		return Polynomial{}, e
	}
	if e = (m.som).addWithArithmeticReceipt(a, u, receipt); e != nil {
		return Polynomial{}, e
	}
	return m.som.mkWithArithmeticReceipt(false, receipt)
}
func (m *MultivariatePolynomialCore) Mul(p, q Polynomial) (Polynomial, error) {
	return m.mulWithReceipt(p, q, nil)
}
func (m *MultivariatePolynomialCore) mulWithReceipt(p, q Polynomial, receipt *polynomialArithmeticInvocationReceipt) (Polynomial, error) {
	zero := unumeral.NewInt(0)
	return m.mulAddWithReceipt(p, q, &zero, receipt)
}
func (m *MultivariatePolynomialCore) Scale(a *unumeral.Int, t Monomial, p Polynomial) (Polynomial, error) {
	return m.scaleWithReceipt(a, t, p, nil)
}
func (m *MultivariatePolynomialCore) scaleWithReceipt(a *unumeral.Int, t Monomial, p Polynomial, receipt *polynomialArithmeticInvocationReceipt) (Polynomial, error) {
	if e := m.check(); e != nil {
		return Polynomial{}, e
	}
	zero, e := m.modular.IsZero(a)
	if e != nil {
		return Polynomial{}, e
	}
	if zero {
		return m.zero, nil
	}
	one, e := m.modular.IsOne(a)
	if e != nil {
		return Polynomial{}, e
	}
	if one {
		u, e := m.monomials.Unit()
		if e != nil {
			return Polynomial{}, e
		}
		if t == u {
			return p, nil
		}
	}
	if e = polynomialArithmeticSourceAssert(2722, func() (bool, error) {
		n, e := m.cheap.ms.size(m)
		return n == 0, e
	}); e != nil {
		return Polynomial{}, e
	}
	if receipt != nil {
		if e = m.cheap.acquireArithmeticInvocation(receipt); e != nil {
			return Polynomial{}, e
		}
	}
	if e = (m.cheap).addMulWithArithmeticReceipt(a, t, p, receipt); e != nil {
		return Polynomial{}, e
	}
	return m.cheap.mkWithArithmeticReceipt(false, receipt)
}
func (m *MultivariatePolynomialCore) Neg(p Polynomial) (Polynomial, error) {
	return m.negWithReceipt(p, nil)
}
func (m *MultivariatePolynomialCore) negWithReceipt(p Polynomial, receipt *polynomialNegInvocationReceipt) (result Polynomial, cause error) {
	return m.negWithParentReceipts(p, receipt, nil, nil)
}
func (m *MultivariatePolynomialCore) negWithParentReceipts(p Polynomial, receipt *polynomialNegInvocationReceipt, appendReceipt *polynomialCheapAppendInvocationReceipt, factoryReceipt *polynomialFactoryInvocationReceipt) (result Polynomial, cause error) {
	actualReceipt, e := m.beginNegInvocation(receipt)
	if e != nil {
		return Polynomial{}, e
	}
	if appendReceipt == nil {
		m.internalAppend = true
		appendReceipt = &m.negAppend
	}
	if factoryReceipt == nil {
		m.internalFactory = true
		factoryReceipt = &m.negFactory
	}
	defer func() {
		m.negativeExecuting = false
		m.negativeBodyCause = cause
		// Only the actual factory leaf receipt owns an untransferred raw result.
		// Even an empty input reaches mk_core; only its actual factory
		// receipt owns that new raw result until source NUM DT completes.
		m.negativeResult = Polynomial{}
		if e := m.finishNegSource(); e != nil {
			result = Polynomial{}
			cause = polynomialNewtonJoin(cause, e)
		}
		if cause == nil && m.internalFactory && m.matchesFactoryInvocation(factoryReceipt) {
			if e := m.cheap.acknowledgeFactoryAppendTransfer(factoryReceipt.invocation); e != nil {
				result = Polynomial{}
				cause = e
				return
			}
			if e := m.transferFactoryResult(factoryReceipt, result); e != nil {
				result = Polynomial{}
				cause = e
				return
			}
		}
		if !m.directPending(m) {
			m.negativeActive = false
		}
	}()
	n, e := m.readWord(p, 8)
	if e != nil {
		return Polynomial{}, e
	}
	for i := uint32(0); i < n; i++ {
		a, e := m.coefficientCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		e = numericIntegerModularManager_copyWithNegReceipt(m.modular, &m.negativeCoefficient, &a, actualReceipt)
		if e != nil {
			return Polynomial{}, e
		}
		e = numericIntegerModularManager_negWithNegReceipt(m.modular, &m.negativeCoefficient, actualReceipt)
		if e != nil {
			return Polynomial{}, e
		}
		t, e := m.monomialCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		e = m.cheap.addConsumedNegReceipts(&m.negativeCoefficient, t, false, nil, appendReceipt, actualReceipt)
		if e != nil {
			return Polynomial{}, e
		}
	}
	result, e = m.cheap.mkWithAppendReceipt(false, appendReceipt, factoryReceipt)
	return result, e
}
func (m *MultivariatePolynomialCore) MkUnivariateTake(v, n uint32, as []unumeral.Int) (Polynomial, error) {
	if e := m.check(); e != nil {
		return Polynomial{}, e
	}
	return m.mkUnivariateSource(v, n, polynomialUnivariateCoefficients{values: as})
}
