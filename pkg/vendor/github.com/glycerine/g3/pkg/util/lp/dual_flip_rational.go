package lp

// Source: src/util/lp/lp_dual_core_solver.h:172-182;
// lp_dual_core_solver.hpp:375-386,695-730. Concrete retained mpq/mpq.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *RationalDualCoreSolver) SignedSpanOfBoxed(j uint32, out *numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	if m.signedSpan != nil {
		return ErrExactVectorPending
	}
	defer func() { m.stopped = err != nil }()
	low, e := m.dualAtBound(j, false)
	if e != nil {
		return e
	}
	if low {
		return m.BoundSpan(j, out)
	}
	m.signedSpan = NewIndexedRationalVector(m.manager, 1)
	if err = m.BoundSpan(j, &m.signedSpan.Data[0]); err != nil {
		return err
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.priceWork = NewIndexedRationalVector(m.manager, 1)
	if err = m.manager.Copy(&m.priceWork.Data[0], &m.signedSpan.Data[0]); err != nil {
		return err
	}
	if err = m.manager.Neg(&m.priceWork.Data[0]); err != nil {
		return err
	}
	if err = m.manager.Copy(out, &m.priceWork.Data[0]); err != nil {
		return err
	}
	if err = m.priceWork.Close(); err != nil {
		return err
	}
	m.priceWork = nil
	if err = m.signedSpan.Close(); err != nil {
		return err
	}
	m.signedSpan = nil
	return nil
}
func (m *RationalDualCoreSolver) UpdateAWave(delta *numeral.Rational, j uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if m.AWave == nil {
		return ErrBounds
	}
	return m.A.AddColumnToVector(delta, j, m.AWave.Data)
}
func (m *RationalDualCoreSolver) ProcessFlipped() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	if m.processSpan != nil {
		return ErrExactVectorPending
	}
	defer func() { m.stopped = err != nil }()
	if err = m.InitAWaveByZeros(); err != nil {
		return err
	}
	for _, j := range dualOrderedKeys(m.FlippedBoxed) {
		m.processSpan = NewIndexedRationalVector(m.manager, 1)
		if err = m.SignedSpanOfBoxed(j, &m.processSpan.Data[0]); err != nil {
			return err
		}
		if err = m.UpdateAWave(&m.processSpan.Data[0], j); err != nil {
			return err
		}
		if err = m.processSpan.Close(); err != nil {
			return err
		}
		m.processSpan = nil
	}
	return nil
}
func (m *RationalDualCoreSolver) UpdateXBAfterBoundFlips() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if m.Factorization == nil || m.AWave == nil {
		return ErrBounds
	}
	if err = m.Factorization.SolveBy(m.AWave.Data); err != nil {
		return err
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	for i := m.A.RowCount(); i > 0; {
		i--
		if uint64(i) >= uint64(len(*m.Basis)) {
			return ErrBounds
		}
		j := (*m.Basis)[i]
		if uint64(j) >= uint64(len(*m.X)) || uint64(i) >= uint64(len(m.AWave.Data)) {
			return ErrBounds
		}
		if err = m.manager.Sub(&(*m.X)[j], &m.AWave.Data[i], &(*m.X)[j]); err != nil {
			return err
		}
	}
	return nil
}
func (m *RationalDualCoreSolver) updateReducedCostsForFlips() (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	for _, j := range *m.NBasis {
		if uint64(j) >= uint64(len(m.D.Data)) || uint64(j) >= uint64(len(m.PivotRow.Data)) {
			return ErrBounds
		}
		m.priceWork = NewIndexedRationalVector(m.manager, 2)
		if err = etaRationalProduct(m.manager, &m.priceWork.Data[0], &m.priceWork.Data[1], &m.ThetaD, &m.PivotRow.Data[j]); err != nil {
			return err
		}
		if err = m.manager.Sub(&m.D.Data[j], &m.priceWork.Data[1], &m.D.Data[j]); err != nil {
			return err
		}
		if err = m.priceWork.Close(); err != nil {
			return err
		}
		m.priceWork = nil
	}
	p := uint32(m.P)
	if uint64(p) >= uint64(len(m.D.Data)) {
		return ErrBounds
	}
	m.priceWork = NewIndexedRationalVector(m.manager, 1)
	if err = m.manager.Copy(&m.priceWork.Data[0], &m.ThetaD); err != nil {
		return err
	}
	if err = m.manager.Neg(&m.priceWork.Data[0]); err != nil {
		return err
	}
	if err = m.manager.Copy(&m.D.Data[p], &m.priceWork.Data[0]); err != nil {
		return err
	}
	if err = m.priceWork.Close(); err != nil {
		return err
	}
	m.priceWork = nil
	return nil
}
func (m *RationalDualCoreSolver) UpdateDAndXB() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if err = m.updateReducedCostsForFlips(); err != nil {
		return err
	}
	if len(m.FlippedBoxed) > 0 {
		if err = m.ProcessFlipped(); err != nil {
			return err
		}
		return m.UpdateXBAfterBoundFlips()
	}
	return nil
}
func (m *RationalDualCoreSolver) ApplyFlips() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	for _, j := range dualOrderedKeys(m.FlippedBoxed) {
		if sourceDebugEnabled {
			low, e := m.dualAtBound(j, false)
			if e != nil {
				return e
			}
			if !low {
				upper, e := m.dualAtBound(j, true)
				if e != nil {
					return e
				}
				if !upper {
					return ErrAssertion
				}
			}
		}
		low, e := m.dualAtBound(j, false)
		if e != nil {
			return e
		}
		bounds := m.LowBounds
		if low {
			bounds = m.UpperBounds
		}
		if uint64(j) >= uint64(len(*bounds)) {
			return ErrBounds
		}
		if err = m.copyPricingValue(&(*m.X)[j], &(*bounds)[j]); err != nil {
			return err
		}
	}
	return nil
}
func (m *RationalDualCoreSolver) CalculateBetaRPrecisely() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.priceWork = NewIndexedRationalVector(m.manager, 4)
	t, b, tmp, product := &m.priceWork.Data[0], &m.priceWork.Data[1], &m.priceWork.Data[2], &m.priceWork.Data[3]
	for i := m.A.RowCount(); i > 0; {
		i--
		if m.PivotRowB1 == nil || uint64(i) >= uint64(len(m.PivotRowB1.Data)) {
			return ErrBounds
		}
		if err = m.manager.Copy(b, &m.PivotRowB1.Data[i]); err != nil {
			return err
		}
		if err = etaRationalProduct(m.manager, tmp, product, b, b); err != nil {
			return err
		}
		if err = m.manager.Add(t, product, t); err != nil {
			return err
		}
		if err = m.manager.Del(product); err != nil {
			return err
		}
		*product = numeral.NewRational()
		if err = m.manager.Del(b); err != nil {
			return err
		}
		*b = numeral.NewRational()
	}
	if m.Betas == nil || m.R < 0 || uint64(m.R) >= uint64(len(m.Betas.Data)) {
		return ErrBounds
	}
	if err = m.manager.Copy(&m.Betas.Data[m.R], t); err != nil {
		return err
	}
	if err = m.priceWork.Close(); err != nil {
		return err
	}
	m.priceWork = nil
	return nil
}
