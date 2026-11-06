package lp

// Source: src/util/lp/lp_primal_core_solver_tableau.h:262-322.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *PairPrimalCoreSolver) buildLeavingTableauMultiplier(coefficient *numeral.Rational) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.leavingMultiplier = NewIndexedRationalVector(m.manager, 4)
	neg, direction, tmp, product := &m.leavingMultiplier.Data[0], &m.leavingMultiplier.Data[1], &m.leavingMultiplier.Data[2], &m.leavingMultiplier.Data[3]
	if err = m.manager.Copy(neg, coefficient); err != nil {
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

func (m *PairPrimalCoreSolver) limitLeavingTableauCell(cell ColumnCell, t *RationalPair, unlimited *bool) (j uint32, err error) {
	coefficient, e := m.A.ColumnValue(cell)
	if e != nil {
		return 0, e
	}
	if sourceDebugEnabled {
		zero, e := m.manager.IsZero(coefficient)
		if e != nil {
			return 0, e
		}
		if zero {
			return 0, ErrAssertion
		}
	}
	if uint64(cell.Row) >= uint64(len(*m.Basis)) {
		return 0, ErrBounds
	}
	j = (*m.Basis)[cell.Row]
	if err = m.buildLeavingTableauMultiplier(coefficient); err != nil {
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

// FindLeavingAndTTableau scans stored columns from the beginning. Equal steps
// prefer shorter actual rows; RNG is consumed only for multiple final ties.
func (m *PairPrimalCoreSolver) FindLeavingAndTTableau(entering uint32, t *RationalPair) (leaving int, err error) {
	leaving = -1
	if err = m.ready(); err != nil {
		return
	}
	defer func() { m.stopped = err != nil }()
	if t == nil {
		return -1, ErrBounds
	}
	unlimited := true
	rowMinNZ := m.A.ColumnCount() + 1
	m.LeavingCandidates = m.LeavingCandidates[:0]
	if uint64(entering) >= uint64(len(m.A.Columns)) {
		return -1, ErrBounds
	}
	column := m.A.Columns[entering]
	size := len(column)
	k := 0
	for ; k < size && unlimited; k++ {
		cell := column[k]
		j, e := m.limitLeavingTableauCell(cell, t, &unlimited)
		if e != nil {
			return -1, e
		}
		if !unlimited {
			m.LeavingCandidates = append(m.LeavingCandidates, j)
			if uint64(cell.Row) >= uint64(len(m.A.Rows)) {
				return -1, ErrBounds
			}
			rowMinNZ = uint32(len(m.A.Rows[cell.Row]))
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
	m.leavingRatio = NewIndexedPairVector(m.manager, 1)
	ratio := &m.leavingRatio.Data[0]
	for ; k < size; k++ {
		cell := column[k]
		unlimited = true
		j, e := m.limitLeavingTableauCell(cell, ratio, &unlimited)
		if e != nil {
			return -1, e
		}
		if unlimited {
			continue
		}
		if uint64(cell.Row) >= uint64(len(m.A.Rows)) {
			return -1, ErrBounds
		}
		nz := uint32(len(m.A.Rows[cell.Row]))
		cmp, e := m.compareLeavingRatio(t)
		if e != nil {
			return -1, e
		}
		if cmp < 0 {
			if err = m.copyLeavingPair(t, ratio); err != nil {
				return
			}
			m.LeavingCandidates = m.LeavingCandidates[:0]
			m.LeavingCandidates = append(m.LeavingCandidates, j)
			rowMinNZ = nz
		} else {
			equal, e := t.Equal(m.manager, ratio)
			if e != nil {
				return -1, e
			}
			if equal && nz < rowMinNZ {
				m.LeavingCandidates = m.LeavingCandidates[:0]
				m.LeavingCandidates = append(m.LeavingCandidates, j)
				// Retain the source's second row-size observation in this branch.
				rowMinNZ = uint32(len(m.A.Rows[cell.Row]))
			} else {
				equal, e = t.Equal(m.manager, ratio)
				if e != nil {
					return -1, e
				}
				if equal && nz == rowMinNZ {
					m.LeavingCandidates = append(m.LeavingCandidates, j)
				}
			}
		}
	}
	if err = m.copyLeavingPair(ratio, t); err != nil {
		return
	}
	jump, e := m.tryJumpToAnotherBoundOnEnteringValue(entering, t, ratio, false)
	if e != nil {
		return -1, e
	}
	if jump {
		if err = m.copyLeavingPair(t, ratio); err != nil {
			return
		}
		if err = m.closeLeavingRatio(); err != nil {
			return
		}
		return int(entering), nil
	}
	if len(m.LeavingCandidates) == 1 {
		leaving = int(m.LeavingCandidates[0])
	} else {
		tie := m.Settings.RandomNext()
		if len(m.LeavingCandidates) == 0 {
			return -1, ErrBounds
		}
		leaving = int(m.LeavingCandidates[int(tie%uint32(len(m.LeavingCandidates)))])
	}
	if err = m.closeLeavingRatio(); err != nil {
		return -1, err
	}
	return leaving, nil
}
