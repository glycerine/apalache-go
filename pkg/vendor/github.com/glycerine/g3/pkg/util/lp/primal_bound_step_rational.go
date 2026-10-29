package lp

// Source: src/util/lp/lp_primal_core_solver.hpp:353-409;
// lp_primal_core_solver.h:337-344.

import "github.com/glycerine/g3/pkg/util/numeral"

// LimitTheta preserves std::min's argument order and assignment prefix. The
// caller owns all scalar values, including a partially assigned theta on failure.
func (m *RationalPrimalCoreSolver) LimitTheta(lim, theta *numeral.Rational, unlimited *bool) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if lim == nil || theta == nil || unlimited == nil {
		return ErrBounds
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	return m.limitThetaValue(lim, theta, unlimited)
}

// The same primal owner calls this while its ratio expression remains live.
func (m *RationalPrimalCoreSolver) limitThetaValue(lim, theta *numeral.Rational, unlimited *bool) (err error) {
	if *unlimited {
		if err = m.manager.Copy(theta, lim); err != nil {
			return err
		}
		*unlimited = false
		return nil
	}
	// std::min(lim, theta) compares theta < lim, choosing lim on equality.
	cmp, e := m.manager.Compare(theta, lim)
	if e != nil {
		return e
	}
	chosen := lim
	if cmp < 0 {
		chosen = theta
	}
	return m.manager.Copy(theta, chosen)
}

func (m *RationalPrimalCoreSolver) assignBoundStep(j uint32, out *numeral.Rational, positive bool) (err error) {
	if out == nil || uint64(j) >= uint64(len(*m.X)) {
		return ErrBounds
	}
	bounds := m.LowBounds
	if positive {
		bounds = m.UpperBounds
	}
	if uint64(j) >= uint64(len(*bounds)) {
		return ErrBounds
	}
	left, right := &(*m.X)[j], &(*bounds)[j]
	if positive {
		left, right = right, left
	}
	m.work = NewIndexedRationalVector(m.manager, 2)
	returned := &m.work.Data[1]
	if err = etaRationalDifference(m.manager, &m.work.Data[0], returned, left, right); err != nil {
		return err
	}
	// The returned subtraction value and assignment into t are distinct
	// source copies. Retain the returned value through a stopped assignment.
	if err = m.manager.Copy(out, returned); err != nil {
		return err
	}
	if err = m.work.Close(); err != nil {
		return err
	}
	m.work = nil
	return nil
}

// TryJumpToAnotherBoundOnEntering publishes t even when theta rejects the
// flip. Unlimited is read-only; theta and t may alias as in the source.
func (m *RationalPrimalCoreSolver) TryJumpToAnotherBoundOnEntering(j uint32, theta, t *numeral.Rational, unlimited bool) (jump bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	return m.tryJumpToAnotherBoundOnEnteringValue(j, theta, t, unlimited)
}

func (m *RationalPrimalCoreSolver) tryJumpToAnotherBoundOnEnteringValue(j uint32, theta, t *numeral.Rational, unlimited bool) (jump bool, err error) {
	if uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return false, ErrBounds
	}
	kind := (*m.ColumnTypes)[j]
	positive := m.SignOfEnteringDelta > 0
	if kind != ColumnBoxed && !(kind == ColumnUpperBound && positive) && !(kind == ColumnLowBound && m.SignOfEnteringDelta < 0) {
		return false, nil
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return false, e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	if err = m.assignBoundStep(j, t, positive); err != nil {
		return false, err
	}
	jump = unlimited
	if !jump {
		if theta == nil {
			return false, ErrBounds
		}
		cmp, e := m.manager.Compare(t, theta)
		if e != nil {
			return false, e
		}
		jump = cmp <= 0
	}
	if jump && sourceDebugEnabled {
		zero := numeral.NewRational()
		cmp, e := m.manager.Compare(t, &zero)
		if e != nil {
			return false, e
		}
		if cmp < 0 {
			return false, ErrAssertion
		}
	}
	return jump, nil
}

// TryJumpToAnotherBoundOnEnteringUnlimited is the distinct source path: only
// boxed columns can flip, and it has no nonnegative-step assertion.
func (m *RationalPrimalCoreSolver) TryJumpToAnotherBoundOnEnteringUnlimited(j uint32, t *numeral.Rational) (jump bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	return m.tryJumpToAnotherBoundOnEnteringUnlimitedValue(j, t)
}

func (m *RationalPrimalCoreSolver) tryJumpToAnotherBoundOnEnteringUnlimitedValue(j uint32, t *numeral.Rational) (jump bool, err error) {
	if uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return false, ErrBounds
	}
	if (*m.ColumnTypes)[j] != ColumnBoxed {
		return false, nil
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return false, e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	if err = m.assignBoundStep(j, t, m.SignOfEnteringDelta > 0); err != nil {
		return false, err
	}
	return true, nil
}
