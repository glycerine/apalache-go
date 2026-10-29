package lp

// Source: src/util/lp/lp_primal_core_solver.h:546-607; precise scalar mpq/mpq branches.

import "github.com/glycerine/g3/pkg/util/numeral"

// Scalar ratio expressions retain each copied-left intermediate and the
// copy returned from compound-reference operators. The multiplier is borrowed;
// the outer by-value dispatcher owns its distinct parameter when applicable.
func (m *RationalPrimalCoreSolver) limitExactBoundRatio(multiplier, x, bound, theta *numeral.Rational, unlimited *bool) error {
	return m.limitExactBoundRatioWithEps(multiplier, x, bound, theta, unlimited, nil, false)
}
func (m *RationalPrimalCoreSolver) limitExactBoundRatioWithEps(multiplier, x, bound, theta *numeral.Rational, unlimited *bool, epsilon *numeral.Rational, positive bool) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.work = NewIndexedRationalVector(m.manager, 4)
	tmp, difference, adjusted, ratio := &m.work.Data[0], &m.work.Data[1], &m.work.Data[2], &m.work.Data[3]
	if err = etaRationalDifference(m.manager, tmp, difference, bound, x); err != nil {
		return err
	}
	numerator := difference
	if epsilon != nil {
		if positive {
			err = etaRationalSum(m.manager, tmp, adjusted, difference, epsilon)
		} else {
			err = etaRationalDifference(m.manager, tmp, adjusted, difference, epsilon)
		}
		if err != nil {
			return err
		}
		numerator = adjusted
	}
	if err = etaRationalQuotient(m.manager, tmp, ratio, numerator, multiplier); err != nil {
		return err
	}
	if err = m.limitThetaValue(ratio, theta, unlimited); err != nil {
		return err
	}
	if err = m.work.Close(); err != nil {
		return err
	}
	m.work = nil
	return nil
}

func (m *RationalPrimalCoreSolver) zeroInfTheta(theta *numeral.Rational, unlimited *bool) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	zero := numeral.NewRational()
	if err = m.manager.Copy(theta, &zero); err != nil {
		return err
	}
	*unlimited = false
	return nil
}

// LimitInfOnBoundMNeg leaves theta untouched when x is already below the
// bound. Equality publishes zero; a value above the bound limits the step.
func (m *RationalPrimalCoreSolver) LimitInfOnBoundMNeg(multiplier *numeral.Rational, x, bound, theta *numeral.Rational, unlimited *bool) (limited bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	return m.limitInfOnBoundMNegValue(multiplier, x, bound, theta, unlimited)
}

func (m *RationalPrimalCoreSolver) limitInfOnBoundMNegValue(multiplier *numeral.Rational, x, bound, theta *numeral.Rational, unlimited *bool) (limited bool, err error) {
	if multiplier == nil || x == nil || bound == nil || theta == nil || unlimited == nil {
		return false, ErrBounds
	}
	if sourceDebugEnabled {
		negative, e := m.manager.IsNeg(multiplier)
		if e != nil {
			return false, e
		}
		if !negative {
			return false, ErrAssertion
		}
	}
	below, e := m.BelowBound(x, bound)
	if e != nil {
		return false, e
	}
	if below {
		return false, nil
	}
	above, e := m.AboveBound(x, bound)
	if e != nil {
		return false, e
	}
	if above {
		err = m.limitExactBoundRatio(multiplier, x, bound, theta, unlimited)
	} else {
		err = m.zeroInfTheta(theta, unlimited)
	}
	return true, err
}

func (m *RationalPrimalCoreSolver) LimitInfOnBoundMPos(multiplier *numeral.Rational, x, bound, theta *numeral.Rational, unlimited *bool) (limited bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	return m.limitInfOnBoundMPosValue(multiplier, x, bound, theta, unlimited)
}

func (m *RationalPrimalCoreSolver) limitInfOnBoundMPosValue(multiplier *numeral.Rational, x, bound, theta *numeral.Rational, unlimited *bool) (limited bool, err error) {
	if multiplier == nil || x == nil || bound == nil || theta == nil || unlimited == nil {
		return false, ErrBounds
	}
	if sourceDebugEnabled {
		positive, e := m.manager.IsPos(multiplier)
		if e != nil {
			return false, e
		}
		if !positive {
			return false, ErrAssertion
		}
	}
	above, e := m.AboveBound(x, bound)
	if e != nil {
		return false, e
	}
	if above {
		return false, nil
	}
	below, e := m.BelowBound(x, bound)
	if e != nil {
		return false, e
	}
	if below {
		err = m.limitExactBoundRatio(multiplier, x, bound, theta, unlimited)
	} else {
		err = m.zeroInfTheta(theta, unlimited)
	}
	return true, err
}

// LimitInfOnLowBoundMPos differs at equality: moving upward from an already
// feasible low bound does not constrain theta or change unlimited.
func (m *RationalPrimalCoreSolver) LimitInfOnLowBoundMPos(multiplier *numeral.Rational, x, bound, theta *numeral.Rational, unlimited *bool) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.limitInfOnLowBoundMPosValue(multiplier, x, bound, theta, unlimited)
}

func (m *RationalPrimalCoreSolver) limitInfOnLowBoundMPosValue(multiplier *numeral.Rational, x, bound, theta *numeral.Rational, unlimited *bool) (err error) {
	if multiplier == nil || x == nil || bound == nil || theta == nil || unlimited == nil {
		return ErrBounds
	}
	if sourceDebugEnabled {
		positive, e := m.manager.IsPos(multiplier)
		if e != nil {
			return e
		}
		if !positive {
			return ErrAssertion
		}
	}
	below, e := m.BelowBound(x, bound)
	if e != nil {
		return e
	}
	if !below {
		return nil
	}
	return m.limitExactBoundRatio(multiplier, x, bound, theta, unlimited)
}
