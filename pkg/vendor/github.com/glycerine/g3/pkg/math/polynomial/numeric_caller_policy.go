package polynomial

// Source: src/math/polynomial/upolynomial.cpp; caller-owned numeric lifetimes.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// invocationBound validates the saved higher invocation without requiring it
// to execute. Actual numeral receipts remain releasable after the source body.
func (r *polynomialNumericRoots) invocationBound() bool {
	if r == nil || r.integer == nil || !r.integer.ValidOwner() {
		return false
	}
	c, z := r.caller, r.integer
	if q := c.squareRoot; q != nil {
		return q.manager != nil && q.manager.matchesSqrtInvocation(q) &&
			q.manager.core.integer == z && c.squareRootInvocation != 0 &&
			q.invocation == c.squareRootInvocation && &q.frame.numeric == r
	}
	if q := c.factorFrontend; q != nil {
		if !(c.factorFrontendHistoryFrame(z) == q) || q.service.manager.config.Integer != r.integer || q.invocation == 0 || q.invocation != c.factorFrontendInvocation || &q.numeric != r {
			return false
		}
		return true
	}
	if q := c.rootUpperBound; q != nil {
		if !(q.matches()) || q.owner.config.Integer != r.integer || q.invocation == 0 || q.invocation != c.rootUpperBoundInvocation || &q.numeric != r {
			return false
		}
		return true
	}
	if q := c.removeOneHalfRoot; q != nil {
		if !(q.matches()) || q.owner.config.Integer != r.integer || q.invocation == 0 || q.invocation != c.removeOneHalfRootInvocation || &q.numeric != r {
			return false
		}
		return true
	}
	if q := c.primePreparation; q != nil {
		if !(q.matches()) || q.owner.manager.config.Integer != r.integer || q.invocation == 0 || q.invocation != c.primePreparationInvocation || &q.numeric != r {
			return false
		}
		return true
	}
	if q := c.recombination; q != nil {
		if !(q.matches()) || q.owner.manager.config.Integer != r.integer || q.invocation == 0 || q.invocation != c.recombinationInvocation || &q.numeric != r {
			return false
		}
		return true
	}
	if q := c.henselCaller; q != nil {
		if !(q.matches()) || q.owner.manager.config.Integer != r.integer || q.invocation == 0 || q.invocation != c.henselCallerInvocation || &q.numeric != r {
			return false
		}
		return true
	}
	if q := c.henselPair; q != nil {
		if !(q.service != nil && q.service.matchesHenselPairInvocation(q)) || q.service.manager.config.Integer != r.integer || q.invocation == 0 || q.invocation != c.henselPairInvocation || &q.frame.numeric != r {
			return false
		}
		return true
	}
	if q := c.divides; q != nil {
		if !(q.core != nil && q.core.self == q.core && q.core.matchesDividesInvocation(q)) || q.core.integer != r.integer || q.invocation == 0 || q.invocation != c.dividesInvocation || &q.frame.numeric != r {
			return false
		}
		return true
	}
	if q := c.scalarExact; q != nil {
		if !(q.core != nil && q.core.self == q.core && q.core.matchesScalarExactDivInvocation(q)) || q.core.integer != r.integer || q.invocation == 0 || q.invocation != c.scalarExactInvocation || &q.frame.numeric != r {
			return false
		}
		return true
	}
	if q := c.exactDiv; q != nil {
		if !(q.core != nil && q.core.self == q.core && q.core.matchesExactDivInvocation(q)) || q.core.integer != r.integer || q.invocation == 0 || q.invocation != c.exactDivInvocation || &q.frame.numeric != r {
			return false
		}
		return true
	}
	if q := c.somNormalize; q != nil {
		if !(q.manager != nil && q.manager.hasSOMNormalizeInvocationHistory(q) && q.frame.active) || q.manager.integer != r.integer || q.invocation == 0 || q.invocation != c.somNormalizeInvocation || &q.buffer.gcdNumeric != r {
			return false
		}
		return true
	}
	if q := c.normalization; q != nil {
		if !(q.manager != nil && q.manager.hasNormalizeInvocationHistory(q) && q.frame.active) || q.manager.core.integer != r.integer || q.invocation == 0 || q.invocation != c.normalizationInvocation || &q.frame.numeric != r {
			return false
		}
		return true
	}
	if q := c.content; q != nil {
		if !(q.manager != nil && q.manager.matchesContentInvocation(q)) || q.manager.integer != r.integer || q.invocation == 0 || q.invocation != c.contentInvocation || &q.frame.numeric != r {
			return false
		}
		return true
	}
	if q := c.scalarDiv; q != nil {
		if !(q.manager != nil && q.manager.self == q.manager && q.manager.matchesScalarDivInvocation(q)) || q.manager.integer != r.integer || q.invocation == 0 || q.invocation != c.scalarDiv.invocation || &q.frame.numeric != r {
			return false
		}
		return true
	}
	return false
}
func (r *polynomialNumericRoots) NumericBound(executing bool) bool {
	valid := r.invocationBound()
	return valid && (!executing || r.caller.roots(r.integer) == r)
}

func (r *polynomialNumericRoots) NumericPrepare(z *unumeral.IntegerManager, phase string) error {
	if r.NumericRoots(z, nil) == nil {
		return unumeral.ErrNumericOwner
	}
	if q := r.caller.upolyDisplayFrame(z); q != nil {
		if e := q.prepareStorage(); e != nil {
			return e
		}
	}
	if e := r.caller.prepareFactorHistory(z); e != nil {
		return e
	}
	return r.caller.prepareFactorFrontendConstantHistory(z)
}
func (r *polynomialNumericRoots) NumericNormalize(m *unumeral.IntegerModularManager, out unumeral.Int) (unumeral.Int, error) {
	if m == nil || r.NumericRoots(m.Integers(), nil) == nil {
		return out, unumeral.ErrNumericOwner
	}
	err := numericIntegerModularManager_normalizeWithSourceContext(m, &out, r.caller)
	return out, err
}
func (r *polynomialNumericRoots) NumericRecord(z *unumeral.IntegerManager, event string, receipt unumeral.NumericInvocationReceipt) {
	if r.NumericRoots(z, nil) == nil {
		return
	}
	c := r.caller
	if receipt.Divides != nil {
		if q := c.divides; q != nil {
			q.frame.boolNumeral = *receipt.Divides
		}
		if q := c.scalarExact; q != nil {
			q.frame.boolNumeral = *receipt.Divides
		}
		if q := c.exactDiv; q != nil {
			q.frame.boolNumeral = *receipt.Divides
		}
		if q := c.factorFrontend; q != nil {
			q.boolNumeral = *receipt.Divides
		}
		if q := c.recombination; q != nil {
			q.boolNumeral = *receipt.Divides
		}
		if q := c.trial; q != nil {
			q.boolNumeral = *receipt.Divides
		}
		if q := c.henselCaller; q != nil {
			q.boolNumeral = *receipt.Divides
		}
		if q := c.henselPair; q != nil {
			q.frame.boolNumeral = *receipt.Divides
		}
		return
	}
	if receipt.PerfectSquare != nil {
		if q := c.squareRoot; q != nil && z.MatchesPerfectSquareInvocation(receipt.PerfectSquare) {
			q.frame.perfectSquare = *receipt.PerfectSquare
		}
		if q := c.factorFrontend; q != nil {
			q.perfectSquare = *receipt.PerfectSquare
		}
		return
	}
	raw := receipt.Raw
	if raw == nil {
		return
	}

	switch event {
	case "division", "extended":
		if q := c.squareRoot; q != nil && event == "division" && z.MatchesRawInvocation(raw) {
			q.frame.divisionRaw = *raw
		}
		c.recordRecombinationRaw(z, raw)
		c.recordRootUpperRaw(z, raw)
		c.recordRemoveOneHalfRaw(z, raw)
		c.recordPrimePreparationRaw(z, raw)
		c.recordFactorFrontendRaw(z, raw)
		c.recordContentRaw(z, raw)
		c.recordNormalizationRaw(z, raw)
		c.recordSOMNormalizationRaw(z, raw)
		division := event == "division"
		numericIntegerManager_bindHenselRaw(z, c, raw, division)
		numericIntegerManager_bindHenselCallerRaw(z, c, raw, division)
		numericIntegerManager_bindExactDivRaw(z, c, raw, division)
		numericIntegerManager_bindScalarExactDivRaw(z, c, raw, division)
		numericIntegerManager_bindDividesRaw(z, c, raw, division)
	case "gcd":
		c.recordRecombinationRaw(z, raw)
		c.recordPrimePreparationRaw(z, raw)
		c.recordFactorFrontendRaw(z, raw)
		c.recordPreparationRaw(z, raw)
		c.recordContentRaw(z, raw)
		c.recordNormalizationRaw(z, raw)
		c.recordSOMNormalizationRaw(z, raw)
	}
}

func (r *polynomialNumericRoots) NumericScalarRawEnabled() bool {
	return r != nil && r.caller.scalarDiv != nil
}
func (r *polynomialNumericRoots) NumericScalarRaw(z *unumeral.IntegerManager, event string) (*unumeral.IntegerScalarRawScope, error) {
	if r.NumericRoots(z, nil) == nil || r.caller.scalarDiv == nil {
		return nil, unumeral.ErrNumericOwner
	}
	s := r.caller.scalarDiv.frame
	switch event {
	case "division":
		return &s.division, nil
	case "extended":
		return &s.extended, nil
	}
	return nil, unumeral.ErrNumericOwner
}

func (r *polynomialNumericRoots) NumericPowerEnabled() bool {
	return r != nil && r.caller.factorFrontend != nil
}
func (r *polynomialNumericRoots) NumericPower(m *unumeral.IntegerModularManager) (unumeral.NumericPowerInvocation, error) {
	if m == nil || r.NumericRoots(m.Integers(), nil) == nil {
		return nil, unumeral.ErrNumericOwner
	}
	q := r.caller.factorFrontendFrame(m.Integers())
	if q == nil || q.service.manager.modular != m {
		return nil, ErrUPolynomialOwner
	}
	s := &q.modularPower
	if s.active || s.executing || s.sourceExecuting || s.ownExecuting || s.invocation == ^uint64(0) {
		return nil, ErrUPolynomialCleanupPending
	}
	*s = upolyFactorModularPowerScope{owner: q, invocation: s.invocation + 1, active: true, executing: true}
	return s, nil
}
func (q *upolyFactorModularPowerScope) NumericPowerValue() *unumeral.Int { return &q.power }
func (q *upolyFactorModularPowerScope) NumericPowerFinish() error {
	q.endEntered = true
	return q.finishEnd()
}
func (q *upolyFactorModularPowerScope) NumericPowerEnd(e error) {
	if e != nil && q.bodyCause == nil {
		q.bodyCause = e
	}
	q.executing = false
}
func (r *polynomialNumericRoots) NumericSourceCause(z *unumeral.IntegerManager) error {
	if r == nil || r.integer != z {
		return nil
	}
	c := r.caller
	if q := c.squareRoot; q != nil && q.manager != nil && q.manager.matchesSqrtInvocationHistory(q) && q.invocation == c.squareRootInvocation {
		return z.RawInvocationSourceCause(&q.frame.divisionRaw)
	}
	if q := c.factorFrontend; q != nil && q.actual() && q.invocation == c.factorFrontendInvocation {
		return q.numericSourceCause()
	}
	if c.divides != nil {
		return numericIntegerManager_dividesNumericSourceCause(z, c.divides)
	}
	if c.scalarExact != nil {
		return numericIntegerManager_scalarExactNumericSourceCause(z, c.scalarExact)
	}
	if c.exactDiv != nil {
		return numericIntegerManager_exactDivNumericSourceCause(z, c.exactDiv)
	}
	if c.somNormalize != nil {
		return numericIntegerManager_somNormalizeNumericSourceCause(z, c.somNormalize)
	}
	if c.normalization != nil {
		return numericIntegerManager_normalizationNumericSourceCause(z, c.normalization)
	}
	return nil
}

// Frontend NUM receipts authorize their original modular owner.
func (r *polynomialNumericRoots) NumericModularOwner(m *unumeral.IntegerModularManager) bool {
	if q := r.caller.squareRoot; q != nil {
		return q.manager != nil && q.manager.core != nil && q.manager.core.modular == m
	}
	if q := r.caller.factorFrontend; q != nil {
		return q.service != nil && q.service.manager != nil && q.service.manager.modular == m
	}
	return true
}
