package lp

// Source: src/util/lp/sparse_matrix.hpp:907-975.

func (m *FloatSparseMatrix) removeRowFromActivePivotsAndShortenColumns(row uint32) (bool, error) {
	arow, err := m.AdjustRow(row)
	if err != nil {
		return false, err
	}
	if uint64(arow) >= uint64(len(m.Rows)) {
		return false, ErrBounds
	}
	for _, iv := range m.Rows[arow] {
		if err := m.pivotQueue.Remove(arow, iv.Index); err != nil {
			return false, err
		}
		m.active--
		col, err := m.AdjustColumnInverse(iv.Index)
		if err != nil {
			return false, err
		}
		if col <= row {
			continue
		}
		if uint64(iv.Index) >= uint64(len(m.Columns)) {
			return false, ErrBounds
		}
		ch := &m.Columns[iv.Index]
		ch.ShortenedMarkovitz++
		if uint64(len(ch.Values)) <= uint64(ch.ShortenedMarkovitz) {
			return false, nil
		}
	}
	return true, nil
}
func (m *FloatSparseMatrix) removePivotColumn(row uint32) error {
	acol, err := m.AdjustColumn(row)
	if err != nil {
		return err
	}
	if uint64(acol) >= uint64(len(m.Columns)) {
		return ErrBounds
	}
	for _, iv := range m.Columns[acol].Values {
		i, err := m.AdjustRowInverse(iv.Index)
		if err != nil {
			return err
		}
		if i >= row {
			if err := m.pivotQueue.Remove(iv.Index, acol); err != nil {
				return err
			}
		}
	}
	return nil
}
func (m *FloatSparseMatrix) updateActivePivots(row uint32) error {
	arow, err := m.AdjustRow(row)
	if err != nil {
		return err
	}
	if uint64(arow) >= uint64(len(m.Rows)) {
		return ErrBounds
	}
	for _, iv := range m.Rows[arow] {
		if uint64(iv.Index) >= uint64(len(m.Columns)) {
			return ErrBounds
		}
		ch := &m.Columns[iv.Index]
		// C++ converts size to int, then unsigned subtraction wraps before int assignment.
		cols := int32(uint32(len(ch.Values)) - ch.ShortenedMarkovitz - 1)
		if cols < 0 {
			if err := sourceAssertionFailure(); err != nil {
				return err
			}
		}
		for _, ivc := range ch.Values {
			i := ivc.Index
			logical, err := m.AdjustRowInverse(i)
			if err != nil {
				return err
			}
			if logical <= row {
				continue
			}
			if uint64(i) >= uint64(len(m.Rows)) {
				return ErrBounds
			}
			if err := m.pivotQueue.Enqueue(i, iv.Index, uint32(len(m.Rows[i]))*uint32(cols)); err != nil {
				return err
			}
		}
	}
	return nil
}

// ShortenActiveMatrix updates the active pivot domain after eliminating a logical
// row. A zero remaining column returns false with completed counter/queue changes
// retained. Stored coefficients are borrowed and never copied or removed here.
func (m *FloatSparseMatrix) ShortenActiveMatrix(row uint32, eta *FloatEtaMatrix) (bool, error) {
	ok, err := m.removeRowFromActivePivotsAndShortenColumns(row)
	if err != nil || !ok {
		return false, err
	}
	if err := m.removePivotColumn(row); err != nil {
		return false, err
	}
	if err := m.updateActivePivots(row); err != nil {
		return false, err
	}
	if eta == nil {
		return true, nil
	}
	for _, entry := range eta.Column.Data {
		physical, err := m.AdjustRow(entry.Index)
		if err != nil {
			return false, err
		}
		if uint64(physical) >= uint64(len(m.Rows)) {
			return false, ErrBounds
		}
		rnz := uint32(len(m.Rows[physical]))
		for _, iv := range m.Rows[physical] {
			if uint64(iv.Index) >= uint64(len(m.Columns)) {
				return false, ErrBounds
			}
			ch := &m.Columns[iv.Index]
			cnz := int32(uint32(len(ch.Values)) - ch.ShortenedMarkovitz - 1)
			if cnz < 0 {
				if err := sourceAssertionFailure(); err != nil {
					return false, err
				}
			}
			if err := m.pivotQueue.Enqueue(physical, iv.Index, rnz*uint32(cnz)); err != nil {
				return false, err
			}
		}
	}
	return true, nil
}

// PivotWithEta applies eta's row eliminations, divides the pivot row by its
// borrowed diagonal, and then shortens the active matrix. A failed child retains
// its own mutation prefix and numerical ownership.
// Source: src/util/lp/sparse_matrix.hpp:228-243.
func (m *FloatSparseMatrix) PivotWithEta(row uint32, eta *FloatEtaMatrix, settings *Settings) (bool, error) {
	for k := range eta.Column.Data {
		entry := &eta.Column.Data[k]
		ok, err := m.PivotRowToRow(row, &entry.Value, entry.Index, settings)
		if err != nil || !ok {
			return false, err
		}
	}
	if err := m.DivideRow(row, &eta.Diagonal, settings); err != nil {
		return false, err
	}
	return m.ShortenActiveMatrix(row, eta)
}
