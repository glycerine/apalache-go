package lp

// Source: src/util/lp/lp_primal_core_solver.h:346-366,685-742.

import "github.com/glycerine/g3/pkg/util/numeral"

// The source multiplier is passed by value. Its copy survives all child
// expressions and caller-output publication, including an output/scalar alias.
func (m *RationalPrimalCoreSolver) copyBasisMultiplier(multiplier *numeral.Rational) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.basisMultiplier = NewIndexedRationalVector(m.manager, 1)
	return m.manager.Copy(&m.basisMultiplier.Data[0], multiplier)
}

// LimitThetaOnBasisColumn dispatches on the source infeasibility set, rather
// than recomputing feasibility from X. The final clamp also applies to free
// columns and directions that impose no new limit, when theta is limited.
func (m *RationalPrimalCoreSolver) LimitThetaOnBasisColumn(j uint32, multiplier *numeral.Rational, theta *numeral.Rational, unlimited *bool) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.limitThetaOnBasisColumnValue(j, multiplier, theta, unlimited)
}

func (m *RationalPrimalCoreSolver) limitThetaOnBasisColumnValue(j uint32, multiplier *numeral.Rational, theta *numeral.Rational, unlimited *bool) (err error) {
	if multiplier == nil || theta == nil || unlimited == nil || uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return ErrBounds
	}
	if err = m.copyBasisMultiplier(multiplier); err != nil {
		return err
	}
	d := &m.basisMultiplier.Data[0]
	kind := (*m.ColumnTypes)[j]
	if kind == ColumnUpperBound || kind == ColumnLowBound || kind == ColumnBoxed || kind == ColumnFixed {
		sign, e := m.manager.Sign(d)
		if e != nil {
			return e
		}
		feasible := m.CurrentXIsFeasible()
		switch kind {
		case ColumnUpperBound:
			if feasible {
				if sign > 0 {
					err = m.limitFeasibleThetaValue(j, d, theta, unlimited, true)
				}
			} else {
				if uint64(j) >= uint64(len(*m.X)) || uint64(j) >= uint64(len(*m.UpperBounds)) {
					return ErrBounds
				}
				x, bound := &(*m.X)[j], &(*m.UpperBounds)[j]
				if sign > 0 {
					_, err = m.limitInfOnBoundMPosValue(d, x, bound, theta, unlimited)
				} else {
					err = m.limitInfOnUpperBoundMNegValue(d, x, bound, theta, unlimited)
				}
			}
		case ColumnLowBound:
			if feasible {
				if sign < 0 {
					err = m.limitFeasibleThetaValue(j, d, theta, unlimited, false)
				}
			} else {
				if uint64(j) >= uint64(len(*m.X)) || uint64(j) >= uint64(len(*m.LowBounds)) {
					return ErrBounds
				}
				x, bound := &(*m.X)[j], &(*m.LowBounds)[j]
				if sign < 0 {
					_, err = m.limitInfOnBoundMNegValue(d, x, bound, theta, unlimited)
				} else {
					err = m.limitInfOnLowBoundMPosValue(d, x, bound, theta, unlimited)
				}
			}
		case ColumnBoxed, ColumnFixed:
			if feasible {
				err = m.limitFeasibleThetaValue(j, d, theta, unlimited, sign > 0)
			} else {
				err = m.limitBoxedThetaValue(j, d, theta, unlimited, sign > 0)
			}
		}
		if err != nil {
			return err
		}
	} else if kind != ColumnFree && sourceDebugEnabled {
		return ErrAssertion
	}
	if err = m.clampBasisTheta(theta, *unlimited); err != nil {
		return err
	}
	if err = m.basisMultiplier.Close(); err != nil {
		return err
	}
	m.basisMultiplier = nil
	return nil
}

func (m *RationalPrimalCoreSolver) clampBasisTheta(theta *numeral.Rational, unlimited bool) (err error) {
	if unlimited {
		return nil
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
		return m.manager.Copy(theta, &zero)
	}
	return nil
}
