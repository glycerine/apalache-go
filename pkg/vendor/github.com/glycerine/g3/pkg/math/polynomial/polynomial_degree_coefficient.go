package polynomial

// Source: src/math/polynomial/polynomial.cpp:1697-1818,3264-3338,3551-3553.

import (
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *MultivariatePolynomialCore) IsZero(p Polynomial) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	n, e := m.readWord(p, 8)
	return n == 0, e
}
func (m *MultivariatePolynomialCore) IsConstant(p Polynomial) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	return m.isConstantCore(p)
}
func (m *MultivariatePolynomialCore) isConstantCore(p Polynomial) (bool, error) {
	n, e := m.readWord(p, 8)
	zero := n == 0

	if e != nil || zero {
		return zero, e
	}
	n, e = m.readWord(p, 8)
	if e != nil || n != 1 {
		return false, e
	}
	term, e := m.monomialCore(p, 0)
	if e != nil {
		return false, e
	}
	sz, e := m.monomials.sizeCore(term)
	return sz == 0, e
}
func (m *MultivariatePolynomialCore) MaxVariable(p Polynomial) (uint32, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	n, e := m.readWord(p, 8)
	if e != nil {
		return 0, e
	}
	if n == 0 {
		return ^uint32(0), nil
	}
	term, e := m.monomialCore(p, 0)
	if e != nil {
		return 0, e
	}
	return m.monomials.MaxVariable(term)
}
func (m *MultivariatePolynomialCore) Degree(p Polynomial, x uint32) (uint32, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	n, e := m.readWord(p, 8)
	if e != nil || n == 0 {
		return 0, e
	}
	term, e := m.monomialCore(p, 0)
	if e != nil {
		return 0, e
	}
	sz, e := m.monomials.sizeCore(term)
	if e != nil || sz == 0 {
		return 0, e
	}
	v, e := m.monomials.varCore(term, sz-1)
	if e != nil {
		return 0, e
	}
	if v == x {
		return m.monomials.degreeAtCore(term, sz-1)
	}
	var result uint32
	for i := uint32(0); i < n; i++ {
		term, e = m.monomialCore(p, i)
		if e != nil {
			return 0, e
		}
		d, e := m.monomials.DegreeOf(term, x)
		if e != nil {
			return 0, e
		}
		if d > result {
			result = d
		}
	}
	return result, nil
}
func (m *MultivariatePolynomialCore) coefficientAssertions(x uint32, reduct bool) error {
	line := int32(3265)
	if reduct {
		line = 3298
	}
	if e := polynomialCoefficientSourceAssert(line, "is_valid(x)", func() (bool, error) { return m.monomials.IsValidVar(x) }); e != nil {
		return e
	}
	line++
	if e := polynomialCoefficientSourceAssert(line, "m_cheap_som_buffer.empty()", func() (bool, error) {
		n, x := m.cheap.ms.size(m)
		return n == 0, x
	}); e != nil {
		return e
	}
	if reduct {
		if e := polynomialCoefficientSourceAssert(3300, "m_cheap_som_buffer2.empty()", func() (bool, error) {
			n, x := m.cheap2.ms.size(m)
			return n == 0, x
		}); e != nil {
			return e
		}
	}
	return nil
}
func (m *MultivariatePolynomialCore) generatedRemoveVariable(term Monomial, x uint32) (Monomial, error) {
	return m.generatedRemoveVariableWithCoefficient(term, x, nil)
}
func (m *MultivariatePolynomialCore) generatedRemoveVariableWithCoefficient(term Monomial, x uint32, scope *polynomialCoefficientScope) (Monomial, error) {
	return m.generatedRemoveVariableWithOutcome(term, x, scope, nil)
}
func (m *MultivariatePolynomialCore) generatedRemoveVariableWithOutcome(term Monomial, x uint32, scope *polynomialCoefficientScope, result *monomialFactoryOutcome) (Monomial, error) {
	if m.generatedActive {
		return Monomial{}, ErrPolynomialPending
	}
	outcome := monomialFactoryOutcome{}
	a, e := m.monomials.removeVariableCreated(term, x, &outcome)
	if result != nil {
		*result = outcome
	}
	if scope != nil && scope == &m.coefficientInvocation && scope.active && scope.executing {
		for i, root := range outcome.allocator {
			if root != (umemory.Block{}) {
				scope.allocator[i] = root
			}
		}
		if e != nil && outcome.started && m.monomials.creation.generation == outcome.generation {
			scope.calleeGeneration = outcome.generation
			m.monomials.creation.coefficientParent = scope
			m.monomials.creation.coefficientInvocation = scope.invocation
		}
		if e == nil && outcome.created {
			scope.generated = a
		}
	}
	if e != nil && outcome.started {
		m.recordGeneratedCallee(outcome)
	}
	if e == nil && outcome.created {
		m.generatedMonomial = a
		if s := m.compositionOwner; s != nil && s.executing {
			s.coreGenerated = a
		}
		m.generatedActive = true
		m.generatedPSC = m.pscExecutingOwner()
		if m.translation.executing {
			m.translation.generated = a
		}
	}
	return a, e
}
func (m *MultivariatePolynomialCore) Coeff(p Polynomial, x, k uint32) (Polynomial, error) {
	return m.coeffCore(p, x, k, nil)
}
func (m *MultivariatePolynomialCore) CoeffWithReduct(p Polynomial, x, k uint32, reduct *PolynomialRef) (Polynomial, error) {
	if reduct == nil || reduct.owner != m {
		return Polynomial{}, ErrPolynomialOwner
	}
	if e := reduct.open(); e != nil {
		return Polynomial{}, e
	}
	return m.coeffCore(p, x, k, &reduct.root)
}
func (m *MultivariatePolynomialCore) coeffCore(p Polynomial, x, k uint32, reduct *polynomialReference) (Polynomial, error) {
	return m.coeffCoreWithReceipts(p, x, k, reduct, nil, nil, nil)
}
func (m *MultivariatePolynomialCore) coeffWithReceipts(p Polynomial, x, k uint32, receipt *polynomialCoefficientInvocationReceipt, appendReceipt *polynomialCheapAppendInvocationReceipt, factoryReceipt *polynomialFactoryInvocationReceipt) (Polynomial, error) {
	return m.coeffCoreWithReceipts(p, x, k, nil, receipt, appendReceipt, factoryReceipt)
}
func (m *MultivariatePolynomialCore) coeffCoreWithReceipts(p Polynomial, x, k uint32, reduct *polynomialReference, receipt *polynomialCoefficientInvocationReceipt, appendReceipt *polynomialCheapAppendInvocationReceipt, factoryReceipt *polynomialFactoryInvocationReceipt) (result Polynomial, cause error) {
	if e := m.check(); e != nil {
		return Polynomial{}, e
	}
	if e := m.coefficientAssertions(x, reduct != nil); e != nil {
		return Polynomial{}, e
	}
	if reduct == nil {
		if e := m.traceCoefficientPolynomial(p, x, k); e != nil {
			return Polynomial{}, e
		}
	}
	n, e := m.readWord(p, 8)
	if e != nil {
		return Polynomial{}, e
	}
	var scope *polynomialCoefficientScope
	if receipt != nil {
		scope = &m.coefficientInvocation
		if scope.active || scope.bufferActive || scope.invocation == ^uint64(0) {
			return Polynomial{}, ErrPolynomialPending
		}
		*scope = polynomialCoefficientScope{active: true, executing: true, invocation: scope.invocation + 1}
		*receipt = polynomialCoefficientInvocationReceipt{manager: m, frame: scope, invocation: scope.invocation}
		defer func() {
			scope.executing = false
			_ = m.coefficientInvocationSourceCause(receipt)
			if cause == nil && scope.generated == (Monomial{}) && scope.calleeGeneration == 0 && scope.allocator == ([3]umemory.Block{}) {
				scope.active = false
			}
		}()
	}
	for i := uint32(0); i < n; i++ {
		term, e := m.monomialCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		d, e := m.monomials.DegreeOf(term, x)
		if e != nil {
			return Polynomial{}, e
		}
		if d == k {
			a, e := m.coefficientCore(p, i)
			if e != nil {
				return Polynomial{}, e
			}
			term, e = m.generatedRemoveVariableWithCoefficient(term, x, scope)
			if e != nil {
				return Polynomial{}, e
			}
			if e = m.coefficientSourceAdd(&m.cheap, &a, term, scope, receipt, appendReceipt, 0); e != nil {
				return Polynomial{}, e
			}
		} else if reduct != nil {
			a, e := m.coefficientCore(p, i)
			if e != nil {
				return Polynomial{}, e
			}
			if e = m.coefficientSourceAdd(&m.cheap2, &a, term, scope, receipt, nil, 1); e != nil {
				return Polynomial{}, e
			}
		}
	}
	if reduct != nil {
		r, e := m.coefficientSourceMK(&m.cheap2, scope, nil, nil, 1)
		if e != nil {
			return Polynomial{}, e
		}
		if e = m.coefficientAssignReduct(reduct, r, scope); e != nil {
			return Polynomial{}, e
		}
	}
	return m.coefficientSourceMK(&m.cheap, scope, appendReceipt, factoryReceipt, 0)
}
func (m *MultivariatePolynomialCore) LeadingCoefficient(p Polynomial, x uint32) (Polynomial, error) {
	d, e := m.Degree(p, x)
	if e != nil {
		return Polynomial{}, e
	}
	return m.Coeff(p, x, d)
}
func (m *MultivariatePolynomialCore) ConstantCoefficient(p Polynomial, x, k uint32, c *unumeral.Int) (bool, error) {
	return m.constantCoefficientWithReceipt(p, x, k, c, nil)
}
func (m *MultivariatePolynomialCore) constantCoefficientWithReceipt(p Polynomial, x, k uint32, c *unumeral.Int, receipt *polynomialConstantCoefficientInvocationReceipt) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	if e := polynomialCoefficientSourceAssert(3319, "is_valid(x)", func() (bool, error) { return m.monomials.IsValidVar(x) }); e != nil {
		return false, e
	}
	var frame *polynomialConstantCoefficientScope
	if receipt != nil {
		frame = &m.constantCoefficientInvocation
		if frame.active || frame.invocation == ^uint64(0) {
			return false, ErrPolynomialPending
		}
		*frame = polynomialConstantCoefficientScope{active: true, executing: true, invocation: frame.invocation + 1}
		*receipt = polynomialConstantCoefficientInvocationReceipt{m, frame, frame.invocation}
		defer func() {
			frame.executing = false
			if !m.integer.MatchesCopyAllocationInvocation(&frame.copy) && !m.modular.MatchesPNormalizeInvocation(&frame.normalize) {
				frame.active = false
			}
		}()
	}
	if e := m.modular.Reset(c); e != nil {
		return false, e
	}
	n, e := m.readWord(p, 8)
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < n; i++ {
		term, e := m.monomialCore(p, i)
		if e != nil {
			return false, e
		}
		d, e := m.monomials.DegreeOf(term, x)
		if e != nil {
			return false, e
		}
		if d == k {
			sz, e := m.monomials.sizeCore(term)
			if e != nil {
				return false, e
			}
			if (k > 0 && sz > 1) || (k == 0 && sz > 0) {
				return false, nil
			}
			a, e := m.coefficientCore(p, i)
			if e != nil {
				return false, e
			}
			var child *unumeral.IntegerCopyAllocationReceipt
			if frame != nil {
				child = &frame.copy
			}
			var normalized *unumeral.IntegerNormalizeInvocationReceipt
			if frame != nil {
				normalized = &frame.normalize
			}
			if e = m.modular.CopyFromOwnerTracked(c, m.integer, &a, child, normalized); e != nil {
				return false, e
			}
		}
	}
	return true, nil
}

func (m *MultivariatePolynomialCore) IsUnivariate(p Polynomial) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	n, e := m.readWord(p, 8)
	if e != nil {
		return false, e
	}
	constant, e := m.IsConstant(p)
	if e != nil || constant {
		return constant, e
	}
	// The source obtains m(0) before max_var, even though the loop overwrites it.
	if _, e = m.monomialCore(p, 0); e != nil {
		return false, e
	}
	x, e := m.MaxVariable(p)
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < n; i++ {
		term, e := m.monomialCore(p, i)
		if e != nil {
			return false, e
		}
		size, e := m.monomials.sizeCore(term)
		if e != nil {
			return false, e
		}
		if size == 1 {
			v, e := m.monomials.varCore(term, 0)
			if e != nil {
				return false, e
			}
			if v == x {
				continue
			}
		}
		// This is a separate source read after the first short-circuit condition.
		size, e = m.monomials.sizeCore(term)
		if e != nil {
			return false, e
		}
		if size == 0 {
			continue
		}
		return false, nil
	}
	return true, nil
}
func (m *MultivariatePolynomialCore) IsLinear(p Polynomial) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	n, e := m.readWord(p, 8)
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < n; i++ {
		term, e := m.monomialCore(p, i)
		if e != nil {
			return false, e
		}
		size, e := m.monomials.sizeCore(term)
		if e != nil {
			return false, e
		}
		if size == 0 {
			continue
		}
		size, e = m.monomials.sizeCore(term)
		if e != nil {
			return false, e
		}
		if size == 1 {
			degree, e := m.monomials.degreeAtCore(term, 0)
			if e != nil {
				return false, e
			}
			if degree == 1 {
				continue
			}
		}
		return false, nil
	}
	return true, nil
}
func (m *MultivariatePolynomialCore) TotalDegree(p Polynomial) (uint32, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	n, e := m.readWord(p, 8)
	if e != nil {
		return 0, e
	}
	result := uint32(0)
	for i := uint32(0); i < n; i++ {
		term, e := m.monomialCore(p, i)
		if e != nil {
			return 0, e
		}
		d, e := m.monomials.degreeCore(term)
		if e != nil {
			return 0, e
		}
		if d > result {
			result = d
		}
	}
	return result, nil
}

// UnivariateCoefficient returns the borrowed scalar numeral, not an owning copy.
func (m *MultivariatePolynomialCore) UnivariateCoefficient(p Polynomial, k uint32) (unumeral.Int, error) {
	if e := m.check(); e != nil {
		return unumeral.Int{}, e
	}
	if e := polynomialCoefficientSourceAssert(1736, "is_univariate(p)", func() (bool, error) {
		return m.IsUnivariate(p)
	}); e != nil {
		return unumeral.Int{}, e
	}
	n, e := m.readWord(p, 8)
	if e != nil {
		return unumeral.Int{}, e
	}
	for i := uint32(0); i < n; i++ {
		term, e := m.monomialCore(p, i)
		if e != nil {
			return unumeral.Int{}, e
		}
		d, e := m.monomials.degreeCore(term)
		if e != nil {
			return unumeral.Int{}, e
		}
		if d == k {
			return m.coefficientCore(p, i)
		}
	}
	return unumeral.Int{}, nil
}
