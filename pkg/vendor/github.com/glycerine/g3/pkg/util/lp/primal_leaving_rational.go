package lp

// Source: src/util/lp/lp_primal_core_solver.hpp:411-475.

import "github.com/glycerine/g3/pkg/util/numeral"

// The full -ed*direction expression stays alive through the by-value basis
// dispatcher call. Each numerical owner captures only its own operations.
func (m *RationalPrimalCoreSolver) buildLeavingMultiplier(i uint32) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.leavingMultiplier = NewIndexedRationalVector(m.manager, 4)
	neg, direction, tmp, product := &m.leavingMultiplier.Data[0], &m.leavingMultiplier.Data[1], &m.leavingMultiplier.Data[2], &m.leavingMultiplier.Data[3]
	if err = m.manager.Copy(neg, &m.Ed.Data[i]); err != nil {
		return err
	}
	if err = m.manager.Neg(neg); err != nil {
		return err
	}
	if err = m.manager.SetInt32(direction, m.SignOfEnteringDelta); err != nil {
		return err
	}
	return etaRationalProduct(m.manager, tmp, product, neg, direction)
}

func (m *RationalPrimalCoreSolver) limitLeavingRow(i uint32, t *numeral.Rational, unlimited *bool) (j uint32, err error) {
	if uint64(i) >= uint64(len(m.Ed.Data)) {
		return 0, ErrBounds
	}
	if sourceDebugEnabled {
		zero, e := m.manager.IsZero(&m.Ed.Data[i])
		if e != nil {
			return 0, e
		}
		if zero {
			return 0, ErrAssertion
		}
	}
	if uint64(i) >= uint64(len(*m.Basis)) {
		return 0, ErrBounds
	}
	j = (*m.Basis)[i]
	if err = m.buildLeavingMultiplier(i); err != nil {
		return j, err
	}
	if err = m.limitThetaOnBasisColumnValue(j, &m.leavingMultiplier.Data[3], t, unlimited); err != nil {
		return j, err
	}
	if err = m.leavingMultiplier.Close(); err != nil {
		return j, err
	}
	m.leavingMultiplier = nil
	return j, nil
}

func (m *RationalPrimalCoreSolver) copyLeavingRational(dst, src *numeral.Rational) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	return m.manager.Copy(dst, src)
}
func (m *RationalPrimalCoreSolver) compareLeavingRatio(t *numeral.Rational) (cmp int, err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return 0, e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	return m.manager.Compare(&m.leavingRatio.Data[0], t)
}
func (m *RationalPrimalCoreSolver) closeLeavingRatio() error {
	if err := m.leavingRatio.Close(); err != nil {
		return err
	}
	m.leavingRatio = nil
	return nil
}

// FindLeavingAndTPrecise scans the stored indexed entering column in circular
// order. Equal steps prefer sparser rows, then select randomly among all ties.
func (m *RationalPrimalCoreSolver) FindLeavingAndTPrecise(entering uint32, t *numeral.Rational) (leaving int, err error) {
	leaving = -1
	if err = m.ready(); err != nil {
		return
	}
	defer func() { m.stopped = err != nil }()
	if m.Settings.UseBreakpointsInFeasibilitySearch && !m.CurrentXIsFeasible() {
		return m.FindLeavingAndTWithBreakpoints(entering, t)
	}
	if t == nil {
		return -1, ErrBounds
	}
	unlimited := true
	steps := len(m.Ed.Index)
	randomStart := m.Settings.RandomNext()
	// The source divides by zero on an empty index. Keep the RNG prefix and
	// report the invalid domain before clearing previously stored candidates.
	if steps == 0 || uint64(steps) > uint64(^uint32(0)) {
		return -1, ErrBounds
	}
	k := int(randomStart % uint32(steps))
	initial := k
	rowMinNZ := m.A.ColumnCount() + 1
	m.LeavingCandidates = m.LeavingCandidates[:0]
	next := func() {
		k++
		if k == steps {
			k = 0
		}
	}
	for {
		i := m.Ed.Index[k]
		j, e := m.limitLeavingRow(i, t, &unlimited)
		if e != nil {
			return -1, e
		}
		if !unlimited {
			m.LeavingCandidates = append(m.LeavingCandidates, j)
			if uint64(i) >= uint64(len(m.RowsNZ)) {
				return -1, ErrBounds
			}
			rowMinNZ = m.RowsNZ[i]
		}
		next()
		if !unlimited || k == initial {
			break
		}
	}
	if unlimited {
		jump, e := m.tryJumpToAnotherBoundOnEnteringUnlimitedValue(entering, t)
		if e != nil {
			return -1, e
		}
		if jump {
			return int(entering), nil
		}
		return -1, nil
	}
	m.leavingRatio = NewIndexedRationalVector(m.manager, 1)
	ratio := &m.leavingRatio.Data[0]
	for k != initial {
		i := m.Ed.Index[k]
		unlimited = true
		j, e := m.limitLeavingRow(i, ratio, &unlimited)
		if e != nil {
			return -1, e
		}
		if unlimited {
			next()
			continue
		}
		if uint64(i) >= uint64(len(m.RowsNZ)) {
			return -1, ErrBounds
		}
		nz := m.RowsNZ[i]
		cmp, e := m.compareLeavingRatio(t)
		if e != nil {
			return -1, e
		}
		if cmp < 0 {
			if err = m.copyLeavingRational(t, ratio); err != nil {
				return
			}
			m.LeavingCandidates = m.LeavingCandidates[:0]
			m.LeavingCandidates = append(m.LeavingCandidates, j)
			rowMinNZ = nz
		} else {
			// Equality compares canonical numerator/denominator values directly;
			// retain both source expressions instead of reusing the '<' result.
			equal, e := m.manager.Equal(t, ratio)
			if e != nil {
				return -1, e
			}
			if equal && nz < rowMinNZ {
				m.LeavingCandidates = m.LeavingCandidates[:0]
				m.LeavingCandidates = append(m.LeavingCandidates, j)
				rowMinNZ = nz
			} else {
				equal, e = m.manager.Equal(t, ratio)
				if e != nil {
					return -1, e
				}
				if equal && nz == rowMinNZ {
					m.LeavingCandidates = append(m.LeavingCandidates, j)
				}
			}
		}
		next()
	}
	if err = m.copyLeavingRational(ratio, t); err != nil {
		return
	}
	unlimited = false
	jump, e := m.tryJumpToAnotherBoundOnEnteringValue(entering, t, ratio, unlimited)
	if e != nil {
		return -1, e
	}
	if jump {
		if err = m.copyLeavingRational(t, ratio); err != nil {
			return
		}
		if err = m.closeLeavingRatio(); err != nil {
			return
		}
		return int(entering), nil
	}
	tie := m.Settings.RandomNext()
	if len(m.LeavingCandidates) == 0 {
		return -1, ErrBounds
	}
	leaving = int(m.LeavingCandidates[int(tie%uint32(len(m.LeavingCandidates)))])
	if err = m.closeLeavingRatio(); err != nil {
		return -1, err
	}
	return leaving, nil
}
