package lp

// Source: src/util/lp/lp_primal_core_solver.h:329,368;
// src/util/rational.h:449-493,507-511. Concrete mpq/mpq expression.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *RationalPrimalCoreSolver) ZeroHarrisEps() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.manager.SetInt32(&m.ConvertedHarrisEpsilon, 0)
}

// HarrisEpsForBound evaluates (1+abs(bound)/10)*epsilon/3 in scalar
// rational order, preserving each binary expression's returned copy.
// The output represents the returned value and may alias a borrowed input.
func (m *RationalPrimalCoreSolver) HarrisEpsForBound(bound, out *numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if bound == nil || out == nil {
		return ErrBounds
	}
	return m.harrisEpsForBoundValue(bound, out)
}

func (m *RationalPrimalCoreSolver) harrisEpsForBoundValue(bound, out *numeral.Rational) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.work = NewIndexedRationalVector(m.manager, 8)
	tmp, absolute, divided, shifted, scaled := &m.work.Data[0], &m.work.Data[1], &m.work.Data[2], &m.work.Data[3], &m.work.Data[4]
	one, ten, three := &m.work.Data[5], &m.work.Data[6], &m.work.Data[7]
	if err = m.manager.SetInt32(one, 1); err != nil {
		return err
	}
	if err = m.manager.SetInt32(ten, 10); err != nil {
		return err
	}
	if err = m.manager.SetInt32(three, 3); err != nil {
		return err
	}
	if err = m.manager.Copy(absolute, bound); err != nil {
		return err
	}
	if err = m.manager.Abs(absolute); err != nil {
		return err
	}
	if err = etaRationalQuotient(m.manager, tmp, divided, absolute, ten); err != nil {
		return err
	}
	// Scalar int+rational keeps the converted integer on the left.
	if err = etaRationalSum(m.manager, tmp, shifted, one, divided); err != nil {
		return err
	}
	if err = etaRationalProduct(m.manager, tmp, scaled, shifted, &m.ConvertedHarrisEpsilon); err != nil {
		return err
	}
	if err = etaRationalQuotient(m.manager, tmp, out, scaled, three); err != nil {
		return err
	}
	if err = m.work.Close(); err != nil {
		return err
	}
	m.work = nil
	return nil
}
