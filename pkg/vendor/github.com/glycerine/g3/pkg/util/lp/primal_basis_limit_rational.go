package lp

// Source: src/util/lp/lp_primal_core_solver.h:539-544,612-657,671-679.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *RationalPrimalCoreSolver) prepareStepEpsilon(bound *numeral.Rational) error {
	m.stepEpsilon = NewIndexedRationalVector(m.manager, 1)
	return m.harrisEpsForBoundValue(bound, &m.stepEpsilon.Data[0])
}
func (m *RationalPrimalCoreSolver) releaseStepEpsilon() error {
	if err := m.stepEpsilon.Close(); err != nil {
		return err
	}
	m.stepEpsilon = nil
	return nil
}

// LimitInfOnUpperBoundMNeg computes epsilon before testing the bound, as in
// source; equality leaves the limit unchanged.
func (m *RationalPrimalCoreSolver) LimitInfOnUpperBoundMNeg(multiplier *numeral.Rational, x, bound, theta *numeral.Rational, unlimited *bool) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.limitInfOnUpperBoundMNegValue(multiplier, x, bound, theta, unlimited)
}

func (m *RationalPrimalCoreSolver) limitInfOnUpperBoundMNegValue(multiplier *numeral.Rational, x, bound, theta *numeral.Rational, unlimited *bool) (err error) {
	if multiplier == nil || x == nil || bound == nil || theta == nil || unlimited == nil {
		return ErrBounds
	}
	if sourceDebugEnabled {
		negative, e := m.manager.IsNeg(multiplier)
		if e != nil {
			return e
		}
		if !negative {
			return ErrAssertion
		}
	}
	if err = m.prepareStepEpsilon(bound); err != nil {
		return err
	}
	above, e := m.AboveBound(x, bound)
	if e != nil {
		return e
	}
	if above {
		if err = m.limitExactBoundRatioWithEps(multiplier, x, bound, theta, unlimited, &m.stepEpsilon.Data[0], false); err != nil {
			return err
		}
	}
	return m.releaseStepEpsilon()
}

func (m *RationalPrimalCoreSolver) LimitThetaOnBasisColumnForInfCaseMPosBoxed(j uint32, multiplier *numeral.Rational, theta *numeral.Rational, unlimited *bool) (err error) {
	return m.limitBoxedTheta(j, multiplier, theta, unlimited, true)
}
func (m *RationalPrimalCoreSolver) LimitThetaOnBasisColumnForInfCaseMNegBoxed(j uint32, multiplier *numeral.Rational, theta *numeral.Rational, unlimited *bool) (err error) {
	return m.limitBoxedTheta(j, multiplier, theta, unlimited, false)
}

func (m *RationalPrimalCoreSolver) limitBoxedTheta(j uint32, multiplier *numeral.Rational, theta *numeral.Rational, unlimited *bool, positive bool) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.limitBoxedThetaValue(j, multiplier, theta, unlimited, positive)
}

func (m *RationalPrimalCoreSolver) limitBoxedThetaValue(j uint32, multiplier *numeral.Rational, theta *numeral.Rational, unlimited *bool, positive bool) (err error) {
	if multiplier == nil || theta == nil || unlimited == nil || uint64(j) >= uint64(len(*m.X)) || uint64(j) >= uint64(len(*m.LowBounds)) || uint64(j) >= uint64(len(*m.UpperBounds)) {
		return ErrBounds
	}
	x, low, upper := &(*m.X)[j], &(*m.LowBounds)[j], &(*m.UpperBounds)[j]
	var target, epsBound *numeral.Rational
	if positive {
		below, e := m.BelowBound(x, low)
		if e != nil {
			return e
		}
		if below {
			target, epsBound = low, upper // Source uses the upper-bound epsilon.
		} else {
			below, e = m.BelowBound(x, upper)
			if e != nil {
				return e
			}
			if below {
				target, epsBound = upper, upper
			} else {
				above, e := m.AboveBound(x, upper)
				if e != nil {
					return e
				}
				if !above {
					return m.zeroInfTheta(theta, unlimited)
				}
				return nil
			}
		}
	} else {
		above, e := m.AboveBound(x, upper)
		if e != nil {
			return e
		}
		if above {
			target, epsBound = upper, upper
		} else {
			above, e = m.AboveBound(x, low)
			if e != nil {
				return e
			}
			if above {
				target, epsBound = low, low
			} else {
				below, e := m.BelowBound(x, low)
				if e != nil {
					return e
				}
				if !below {
					return m.zeroInfTheta(theta, unlimited)
				}
				return nil
			}
		}
	}
	if err = m.prepareStepEpsilon(epsBound); err != nil {
		return err
	}
	if err = m.limitExactBoundRatioWithEps(multiplier, x, target, theta, unlimited, &m.stepEpsilon.Data[0], positive); err != nil {
		return err
	}
	return m.releaseStepEpsilon()
}

func (m *RationalPrimalCoreSolver) LimitThetaOnBasisColumnForFeasCaseMPosNoCheck(j uint32, multiplier *numeral.Rational, theta *numeral.Rational, unlimited *bool) error {
	return m.limitFeasibleTheta(j, multiplier, theta, unlimited, true)
}
func (m *RationalPrimalCoreSolver) LimitThetaOnBasisColumnForFeasCaseMNegNoCheck(j uint32, multiplier *numeral.Rational, theta *numeral.Rational, unlimited *bool) error {
	return m.limitFeasibleTheta(j, multiplier, theta, unlimited, false)
}
func (m *RationalPrimalCoreSolver) limitFeasibleTheta(j uint32, multiplier *numeral.Rational, theta *numeral.Rational, unlimited *bool, positive bool) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.limitFeasibleThetaValue(j, multiplier, theta, unlimited, positive)
}

func (m *RationalPrimalCoreSolver) limitFeasibleThetaValue(j uint32, multiplier *numeral.Rational, theta *numeral.Rational, unlimited *bool, positive bool) (err error) {
	bounds := m.LowBounds
	if positive {
		bounds = m.UpperBounds
	}
	if multiplier == nil || theta == nil || unlimited == nil || uint64(j) >= uint64(len(*m.X)) || uint64(j) >= uint64(len(*bounds)) {
		return ErrBounds
	}
	if sourceDebugEnabled {
		sign, e := m.manager.Sign(multiplier)
		if e != nil {
			return e
		}
		if (positive && sign <= 0) || (!positive && sign >= 0) {
			return ErrAssertion
		}
	}
	bound := &(*bounds)[j]
	if err = m.prepareStepEpsilon(bound); err != nil {
		return err
	}
	if err = m.limitExactBoundRatioWithEps(multiplier, &(*m.X)[j], bound, theta, unlimited, &m.stepEpsilon.Data[0], positive); err != nil {
		return err
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	zero := numeral.NewRational()
	cmp, e := m.manager.Compare(theta, &zero)
	if e != nil {
		return e
	}
	if cmp < 0 {
		if err = m.manager.Copy(theta, &zero); err != nil {
			return err
		}
	}
	return m.releaseStepEpsilon()
}
