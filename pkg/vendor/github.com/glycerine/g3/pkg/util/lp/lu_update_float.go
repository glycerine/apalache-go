package lp

// Source: src/util/lp/lu.hpp:581-587,775-934, retained double/double.

import "math"

func (m *FloatLU) calculateRWave(start, end uint32) error {
	if start > end {
		m.status = LUDegenerated
		return nil
	}
	if start == end {
		return nil
	}
	if err := m.RWave.Set(start, end); err != nil {
		return err
	}
	for i := start + 1; i <= end; i++ {
		if err := m.RWave.Set(i, i-1); err != nil {
			return err
		}
	}
	if err := m.U.MultiplyRight(&m.RWave.Permutation); err != nil {
		return err
	}
	return m.U.MultiplyReverseLeft(&m.RWave.Permutation)
}

func (m *FloatLU) scanUpdateRow(end uint32) error {
	row, err := m.U.AdjustRow(end)
	if err != nil {
		return err
	}
	if uint64(row) >= uint64(len(m.U.Rows)) {
		return ErrBounds
	}
	for _, iv := range m.U.Rows[row] {
		if iv.Value == 0 {
			continue
		}
		if m.Settings.FloatDropSmall(iv.Value) {
			if err = sourceAssertionFailure(); err != nil {
				return err
			}
		}
		j, err := m.U.AdjustColumnInverse(iv.Index)
		if err != nil {
			return err
		}
		value := iv.Value
		if j < end {
			value = -value
		}
		if err = m.rowEtaWork.SetValue(value, j); err != nil {
			return err
		}
	}
	return nil
}

func (m *FloatLU) solveUpdateRow(start, end uint32) error {
	w := m.rowEtaWork
	for j := start; j < end; j++ {
		if uint64(j) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		v := w.Data[j]
		if v == 0 {
			continue
		}
		row, err := m.U.AdjustRow(j)
		if err != nil {
			return err
		}
		if uint64(row) >= uint64(len(m.U.Rows)) {
			return ErrBounds
		}
		for _, iv := range m.U.Rows[row] {
			col, err := m.U.AdjustColumnInverse(iv.Index)
			if err != nil {
				return err
			}
			if col < j && iv.Value != 0 {
				if err = sourceAssertionFailure(); err != nil {
					return err
				}
			}
			if col == j || iv.Value == 0 {
				continue
			}
			delta := v * iv.Value
			if col < end {
				delta = -v * iv.Value
			}
			if delta == 0 {
				if err = sourceAssertionFailure(); err != nil {
					return err
				}
			}
			if uint64(col) >= uint64(len(w.Data)) {
				return ErrBounds
			}
			if w.Data[col] == 0 {
				if !m.Settings.FloatDropSmall(delta) {
					if err = w.SetValue(delta, col); err != nil {
						return err
					}
				}
			} else {
				w.Data[col] += delta
				if m.Settings.FloatDropSmall(w.Data[col]) {
					w.Data[col] = 0
					w.EraseFromIndex(col)
				}
			}
		}
	}
	return nil
}

func (m *FloatLU) makeUpdateRowEta(start, end uint32, pivotCheck float64) (*FloatRowEtaMatrix, error) {
	if start == end {
		return nil, nil
	}
	if err := m.scanUpdateRow(end); err != nil {
		return nil, err
	}
	if err := m.solveUpdateRow(start, end); err != nil {
		return nil, err
	}
	w := m.rowEtaWork
	if uint64(end) >= uint64(len(w.Data)) {
		return nil, ErrBounds
	}
	if pivotCheck != 0 {
		denom := math.Abs(pivotCheck)
		// std::max(1, NaN) returns its first operand.
		if !(1 < denom) {
			denom = 1
		}
		if !m.Settings.FloatPivotSmall((w.Data[end] - pivotCheck) / denom) {
			m.status = LUDegenerated
			return nil, nil
		}
	}
	eta := NewFloatRowEtaMatrix(start, end, m.Dim)
	for _, j := range w.Index {
		if j >= end {
			continue
		}
		if uint64(j) >= uint64(len(w.Data)) {
			return nil, ErrBounds
		}
		v := w.Data[j]
		if v == 0 {
			continue
		}
		if !m.Settings.FloatDropSmall(v) {
			if err := eta.Append(j, v); err != nil {
				return nil, err
			}
		}
		w.Data[j] = 0
	}
	return eta, nil
}

func (m *FloatLU) finishUpdateBump(start, end uint32) error {
	var diagonal float64
	var err error
	if start < end {
		if uint64(end) >= uint64(len(m.rowEtaWork.Data)) {
			return ErrBounds
		}
		diagonal = m.rowEtaWork.Data[end]
		row, err := m.U.AdjustRow(end)
		if err != nil {
			return err
		}
		if _, err = m.U.SetRowFromWorkVectorAndCleanNotAdjusted(row, m.rowEtaWork, m.Settings); err != nil {
			return err
		}
	} else {
		diagonal, err = m.U.Get(end, end)
		if err != nil {
			return err
		}
	}
	if m.Settings.FloatPivotSmall(diagonal) {
		m.status = LUDegenerated
		return nil
	}
	tail := NewFloatDiagonalMatrix(end, diagonal)
	if sourceDebugEnabled {
		tail.length = m.Dim
	}
	// Source publishes the tail before division and conjugates it afterwards.
	m.Tail = append(m.Tail, tail)
	if err = m.U.DivideRow(end, &diagonal, m.Settings); err != nil {
		return err
	}
	return tail.Conjugate(&m.Q.Permutation)
}

// ReplaceColumn consumes a prepared entering column and replaces the basis
// position leaving. Degeneration is a source status, while Go errors stop the
// owner with completed matrix, permutation and tail changes still published.
func (m *FloatLU) ReplaceColumn(pivotCheck float64, w *IndexedFloatVector, leaving uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finish(&err)
	m.RefactorCounter++
	start, err := m.R.Reverse(leaving)
	if err != nil {
		return err
	}
	if err = m.U.ReplaceColumn(start, w, m.Settings); err != nil {
		return err
	}
	end, err := m.U.LowestRowInColumn(start)
	if err != nil {
		return err
	}
	m.RWave.Init(m.Dim)
	if err = m.calculateRWave(start, end); err != nil {
		return err
	}
	eta, err := m.makeUpdateRowEta(start, end, pivotCheck)
	if err != nil {
		return err
	}
	if m.status == LUDegenerated {
		m.rowEtaWork.ClearAll()
		return nil
	}
	if err = m.Q.MultiplyRight(&m.RWave.Permutation); err != nil {
		return err
	}
	if err = m.R.MultiplyReverseLeft(&m.RWave.Permutation); err != nil {
		return err
	}
	if eta != nil {
		if err = eta.Conjugate(&m.Q.Permutation); err != nil {
			return err
		}
		m.Tail = append(m.Tail, eta)
	}
	return m.finishUpdateBump(start, end)
}
