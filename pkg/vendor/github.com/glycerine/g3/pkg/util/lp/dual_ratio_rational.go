package lp

// Source: src/util/lp/lp_dual_core_solver.hpp:575-599,660-693;
// lp_core_solver_base.h:331-333. Concrete retained mpq/mpq ratio test.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *RationalCoreSolverBase) BoundSpan(j uint32, out *numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if uint64(j) >= uint64(len(*m.LowBounds)) || uint64(j) >= uint64(len(*m.UpperBounds)) {
		return ErrBounds
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.valueWork = NewIndexedRationalVector(m.manager, 1)
	if err = etaRationalDifference(m.manager, &m.valueWork.Data[0], out, &(*m.UpperBounds)[j], &(*m.LowBounds)[j]); err != nil {
		return err
	}
	if err = m.valueWork.Close(); err != nil {
		return err
	}
	m.valueWork = nil
	return nil
}
func (m *RationalDualCoreSolver) TightBreakpointsAreAllBoxed() (boxed bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	q := uint32(m.Q)
	if uint64(q) >= uint64(len(*m.ColumnTypes)) {
		return false, ErrBounds
	}
	if (*m.ColumnTypes)[q] != ColumnBoxed {
		return false, nil
	}
	for _, j := range dualOrderedKeys(m.TightSet) {
		if uint64(j) >= uint64(len(*m.ColumnTypes)) {
			return false, ErrBounds
		}
		if (*m.ColumnTypes)[j] != ColumnBoxed {
			return false, nil
		}
	}
	return true, nil
}
func (m *RationalDualCoreSolver) AddTightBreakpointsAndQToFlippedSet() error {
	if err := m.ready(); err != nil {
		return err
	}
	if m.FlippedBoxed == nil {
		m.FlippedBoxed = make(map[uint32]bool)
	}
	m.FlippedBoxed[uint32(m.Q)] = true
	for _, j := range dualOrderedKeys(m.TightSet) {
		m.FlippedBoxed[j] = true
	}
	return nil
}
func (m *RationalDualCoreSolver) EraseTightBreakpointsAndQFromBreakpointSet() error {
	if err := m.ready(); err != nil {
		return err
	}
	delete(m.BreakpointSet, uint32(m.Q))
	for _, j := range dualOrderedKeys(m.TightSet) {
		delete(m.BreakpointSet, j)
	}
	return nil
}

// flipSpanMagnitude keeps the base-owned span return alive through the actual
// scalar product and copied-input abs expression in the dual owner.
func (m *RationalDualCoreSolver) flipSpanMagnitude(j uint32, out *numeral.Rational) (err error) {
	if m.flipSpan != nil {
		return ErrExactVectorPending
	}
	m.flipSpan = NewIndexedRationalVector(m.manager, 1)
	if err = m.BoundSpan(j, &m.flipSpan.Data[0]); err != nil {
		return err
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.priceWork = NewIndexedRationalVector(m.manager, 3)
	tmp, product, absolute := &m.priceWork.Data[0], &m.priceWork.Data[1], &m.priceWork.Data[2]
	if uint64(j) >= uint64(len(m.PivotRow.Data)) {
		return ErrBounds
	}
	if err = etaRationalProduct(m.manager, tmp, product, &m.flipSpan.Data[0], &m.PivotRow.Data[j]); err != nil {
		return err
	}
	if err = m.manager.Copy(absolute, product); err != nil {
		return err
	}
	if err = m.manager.Abs(absolute); err != nil {
		return err
	}
	if err = m.manager.Copy(out, absolute); err != nil {
		return err
	}
	if err = m.priceWork.Close(); err != nil {
		return err
	}
	m.priceWork = nil
	if err = m.flipSpan.Close(); err != nil {
		return err
	}
	m.flipSpan = nil
	return nil
}
func (m *RationalDualCoreSolver) DeltaLostOnFlipsOfTightBreakpoints(out *numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	if m.flipLossValues != nil {
		return ErrExactVectorPending
	}
	defer func() { m.stopped = err != nil }()
	m.flipLossValues = NewIndexedRationalVector(m.manager, 2)
	ret, term := &m.flipLossValues.Data[0], &m.flipLossValues.Data[1]
	if err = m.flipSpanMagnitude(uint32(m.Q), ret); err != nil {
		return err
	}
	for _, j := range dualOrderedKeys(m.TightSet) {
		if err = m.flipSpanMagnitude(j, term); err != nil {
			return err
		}
		before, e := m.calls.begin(m.manager)
		if e != nil {
			return e
		}
		err = m.manager.Add(ret, term, ret)
		if err = m.calls.after(m.manager, before, err); err != nil {
			return err
		}
		if err = m.manager.Del(term); err != nil {
			return err
		}
		*term = numeral.NewRational()
	}
	if err = m.copyPricingValue(out, ret); err != nil {
		return err
	}
	if err = m.flipLossValues.Close(); err != nil {
		return err
	}
	m.flipLossValues = nil
	return nil
}
func (m *RationalDualCoreSolver) buildRatioTrial(initial int) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.priceWork = NewIndexedRationalVector(m.manager, 4)
	tmp, sign, product, difference := &m.priceWork.Data[0], &m.priceWork.Data[1], &m.priceWork.Data[2], &m.priceWork.Data[3]
	m.ratioTrial = NewIndexedRationalVector(m.manager, 1)
	if err = m.manager.SetInt64(sign, int64(initial)); err != nil {
		return err
	}
	if err = etaRationalProduct(m.manager, tmp, product, &m.ratioLoss.Data[0], sign); err != nil {
		return err
	}
	if err = etaRationalDifference(m.manager, difference, &m.ratioTrial.Data[0], &m.Delta, product); err != nil {
		return err
	}
	if err = m.priceWork.Close(); err != nil {
		return err
	}
	m.priceWork = nil
	if err = m.ratioLoss.Close(); err != nil {
		return err
	}
	m.ratioLoss = nil
	return nil
}
func (m *RationalDualCoreSolver) closeRatioTrial() error {
	if m.ratioTrial != nil {
		if err := m.ratioTrial.Close(); err != nil {
			return err
		}
		m.ratioTrial = nil
	}
	return nil
}
func (m *RationalDualCoreSolver) assignRatioTheta() (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	q := uint32(m.Q)
	if uint64(q) >= uint64(len(m.D.Data)) || uint64(q) >= uint64(len(m.PivotRow.Data)) {
		return ErrBounds
	}
	m.priceWork = NewIndexedRationalVector(m.manager, 1)
	if err = etaRationalQuotient(m.manager, &m.priceWork.Data[0], &m.ThetaD, &m.D.Data[q], &m.PivotRow.Data[q]); err != nil {
		return err
	}
	if err = m.priceWork.Close(); err != nil {
		return err
	}
	m.priceWork = nil
	return nil
}
func (m *RationalDualCoreSolver) RatioTest() (found bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	if m.ratioLoss != nil || m.ratioTrial != nil {
		return false, ErrExactVectorPending
	}
	defer func() { m.stopped = err != nil }()
	sign, e := m.DefineSignOfAlphaR()
	if e != nil {
		return false, e
	}
	m.SignOfAlphaR = sign
	if err = m.FillBreakpointSet(); err != nil {
		return false, err
	}
	m.FlippedBoxed = make(map[uint32]bool)
	zero := numeral.NewRational()
	cmp, e := m.compareDualValues(&m.Delta, &zero)
	if e != nil {
		return false, e
	}
	initial := 1
	if cmp < 0 {
		initial = -1
	}
	for {
		if len(m.BreakpointSet) == 0 {
			if err = m.SetStatusToTentativeDualUnboundedOrDualUnbounded(); err != nil {
				return false, err
			}
			return false, nil
		}
		m.Status = StatusFeasible
		if err = m.FindQAndTightSet(); err != nil {
			return false, err
		}
		boxed, e := m.TightBreakpointsAreAllBoxed()
		if e != nil {
			return false, e
		}
		if !boxed {
			break
		}
		m.ratioLoss = NewIndexedRationalVector(m.manager, 1)
		if err = m.DeltaLostOnFlipsOfTightBreakpoints(&m.ratioLoss.Data[0]); err != nil {
			return false, err
		}
		if err = m.buildRatioTrial(initial); err != nil {
			return false, err
		}
		keeps, e := m.DeltaKeepsTheSign(initial, &m.ratioTrial.Data[0])
		if e != nil {
			return false, e
		}
		if !keeps {
			break
		}
		if len(m.TightSet)+1 == len(m.BreakpointSet) {
			break
		}
		if err = m.AddTightBreakpointsAndQToFlippedSet(); err != nil {
			return false, err
		}
		if err = m.copyPricingValue(&m.Delta, &m.ratioTrial.Data[0]); err != nil {
			return false, err
		}
		if err = m.EraseTightBreakpointsAndQFromBreakpointSet(); err != nil {
			return false, err
		}
		if err = m.closeRatioTrial(); err != nil {
			return false, err
		}
	}
	// The source loop-local del dies on break before theta's expression starts.
	if err = m.closeRatioTrial(); err != nil {
		return false, err
	}
	if err = m.assignRatioTheta(); err != nil {
		return false, err
	}
	return true, nil
}
