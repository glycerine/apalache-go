package lp

// Source: src/util/lp/lu.hpp:581-587,775-934; retained mpq coefficients.

import "github.com/glycerine/g3/pkg/util/numeral"

// Lexical slots: pivot argument, v, unary minus, delta, t, diagonal,
// last-row by-value argument and product expression temporary. Each scalar
// operation captures only its own calls, never delegated child invocations.
func (m *exactLUState) beginUpdate(pivot *numeral.Rational, w *IndexedRationalVector) error {
	if err := m.ready(); err != nil {
		return err
	}
	if w.manager != m.A.manager {
		return ErrExactVectorOwner
	}
	if w.calls.pending() {
		return ErrExactVectorPending
	}
	m.updateWork = NewIndexedRationalVector(m.A.manager, 8)
	// Source value-parameter construction precedes the counter increment.
	return m.updateWork.copySlot(&m.updateWork.Data[0], pivot)
}

func (m *exactLUState) finishUpdate(err *error) {
	if *err == nil && m.updateWork != nil {
		*err = m.updateWork.Close()
		if *err == nil {
			m.updateWork = nil
		}
	}
	m.finish(err)
}

// Resolve failed calls before deleting their borrowed lexical inputs or
// published tails. Ordinary source-order member destruction follows afterwards.
func (m *exactLUState) closeUpdatePending() error {
	if m.updateWork == nil {
		return nil
	}
	if err := m.updateWork.calls.cleanupRaw(m.A.manager); err != nil {
		return err
	}
	if err := m.rowEtaWork.calls.cleanupRaw(m.A.manager); err != nil {
		return err
	}
	if m.U != nil {
		if err := m.U.calls.cleanupRaw(m.A.manager); err != nil {
			return err
		}
	}
	if m.pendingRowEta != nil {
		if err := m.pendingRowEta.Close(); err != nil {
			return err
		}
		m.pendingRowEta = nil
	}
	if m.pendingDiagonal != nil {
		if err := m.pendingDiagonal.Close(); err != nil {
			return err
		}
		m.pendingDiagonal = nil
	}
	return nil
}

func (m *exactLUState) updateNegative(value *numeral.Rational) (err error) {
	w := m.updateWork
	if err = w.copySlot(&w.Data[2], value); err != nil {
		return err
	}
	b, err := w.calls.begin(m.A.manager)
	if err != nil {
		return err
	}
	defer func() { err = w.calls.after(m.A.manager, b, err) }()
	return m.A.manager.Neg(&w.Data[2])
}

func (m *exactLUState) updateProduct(left, right *numeral.Rational) (err error) {
	w := m.updateWork
	b, err := w.calls.begin(m.A.manager)
	if err != nil {
		return err
	}
	defer func() { err = w.calls.after(m.A.manager, b, err) }()
	return etaRationalProduct(m.A.manager, &w.Data[7], &w.Data[3], left, right)
}

func (m *exactLUState) updateAdd(col uint32) (err error) {
	w := m.updateWork
	b, err := w.calls.begin(m.A.manager)
	if err != nil {
		return err
	}
	// The compound assignment publishes before the separate owning t copy.
	err = m.A.manager.Add(&m.rowEtaWork.Data[col], &w.Data[3], &m.rowEtaWork.Data[col])
	err = w.calls.after(m.A.manager, b, err)
	if err != nil {
		return err
	}
	return w.copySlot(&w.Data[4], &m.rowEtaWork.Data[col])
}

func (m *exactLUState) transformUpdateColumn(w *IndexedRationalVector, leaving uint32) (uint32, uint32, error) {
	m.RefactorCounter++
	start, err := m.r.Reverse(leaving)
	if err != nil {
		return 0, 0, err
	}
	if err = m.U.ReplaceColumn(start, w, m.Settings); err != nil {
		return 0, 0, err
	}
	end, err := m.U.LowestRowInColumn(start)
	return start, end, err
}

func (m *exactLUState) calculateExactRWave(start, end uint32, wave *Permutation) error {
	if start > end {
		m.status = LUDegenerated
		return nil
	}
	if start == end {
		return nil
	}
	if err := wave.Set(start, end); err != nil {
		return err
	}
	for i := start + 1; i <= end; i++ {
		if err := wave.Set(i, i-1); err != nil {
			return err
		}
	}
	if err := m.U.MultiplyRight(wave); err != nil {
		return err
	}
	return m.U.MultiplyReverseLeft(wave)
}

func (m *exactLUState) scanExactUpdateRow(end uint32) error {
	row, err := m.U.AdjustRow(end)
	if err != nil {
		return err
	}
	if uint64(row) >= uint64(len(m.U.Rows)) {
		return ErrBounds
	}
	for k := range m.U.Rows[row] {
		iv := &m.U.Rows[row][k]
		zero, err := m.A.manager.IsZero(&iv.Value)
		if err != nil {
			return err
		}
		if zero {
			continue
		}
		col, err := m.U.AdjustColumnInverse(iv.Index)
		if err != nil {
			return err
		}
		if col < end {
			if err = m.updateNegative(&iv.Value); err != nil {
				return err
			}
			if err = m.rowEtaWork.SetValue(&m.updateWork.Data[2], col); err != nil {
				return err
			}
			if err = m.updateWork.resetSlot(&m.updateWork.Data[2]); err != nil {
				return err
			}
		} else if err = m.rowEtaWork.SetValue(&iv.Value, col); err != nil {
			return err
		}
	}
	return nil
}

func (m *exactLUState) solveExactUpdateRow(start, end uint32) error {
	w, local := m.rowEtaWork, m.updateWork
	manager := m.A.manager
	for j := start; j < end; j++ {
		if uint64(j) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		if err := local.copySlot(&local.Data[1], &w.Data[j]); err != nil {
			return err
		}
		zero, err := manager.IsZero(&local.Data[1])
		if err != nil {
			return err
		}
		if !zero {
			row, err := m.U.AdjustRow(j)
			if err != nil {
				return err
			}
			if uint64(row) >= uint64(len(m.U.Rows)) {
				return ErrBounds
			}
			for k := range m.U.Rows[row] {
				iv := &m.U.Rows[row][k]
				col, err := m.U.AdjustColumnInverse(iv.Index)
				if err != nil {
					return err
				}
				zero, err := manager.IsZero(&iv.Value)
				if err != nil {
					return err
				}
				if col < j && !zero {
					if err = sourceAssertionFailure(); err != nil {
						return err
					}
				}
				if col == j || zero {
					continue
				}
				left := &local.Data[1]
				if col < end {
					if err = m.updateNegative(left); err != nil {
						return err
					}
					left = &local.Data[2]
				}
				if err = m.updateProduct(left, &iv.Value); err != nil {
					return err
				}
				if err = local.resetSlot(&local.Data[2]); err != nil {
					return err
				}
				zero, err = manager.IsZero(&local.Data[3])
				if err != nil {
					return err
				}
				if zero {
					if err = sourceAssertionFailure(); err != nil {
						return err
					}
				}
				if uint64(col) >= uint64(len(w.Data)) {
					return ErrBounds
				}
				zero, err = manager.IsZero(&w.Data[col])
				if err != nil {
					return err
				}
				if zero {
					// Exact drop tolerance is zero; a nonzero precise product is retained.
					drop, err := manager.IsZero(&local.Data[3])
					if err != nil {
						return err
					}
					if !drop {
						if err = w.SetValue(&local.Data[3], col); err != nil {
							return err
						}
					}
				} else {
					if err = m.updateAdd(col); err != nil {
						return err
					}
					drop, err := manager.IsZero(&local.Data[4])
					if err != nil {
						return err
					}
					if drop {
						if err = w.resetSlot(&w.Data[col]); err != nil {
							return err
						}
						w.EraseFromIndex(col)
					}
					if err = local.resetSlot(&local.Data[4]); err != nil {
						return err
					}
				}
				if err = local.resetSlot(&local.Data[3]); err != nil {
					return err
				}
			}
		}
		if err = local.resetSlot(&local.Data[1]); err != nil {
			return err
		}
	}
	return nil
}

func (m *exactLUState) makeExactUpdateRowEta(start, end uint32) error {
	if start == end {
		return nil
	}
	if err := m.scanExactUpdateRow(end); err != nil {
		return err
	}
	if err := m.solveExactUpdateRow(start, end); err != nil {
		return err
	}
	// The precise source branch never checks pivotCheck, even when nonzero.
	m.pendingRowEta = NewRationalRowEtaMatrix(m.A.manager, start, end, m.Dim)
	w := m.rowEtaWork
	for _, j := range w.Index {
		if j >= end {
			continue
		}
		if uint64(j) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		zero, err := m.A.manager.IsZero(&w.Data[j])
		if err != nil {
			return err
		}
		if zero {
			continue
		}
		if err = m.pendingRowEta.Append(j, &w.Data[j]); err != nil {
			return err
		}
		if err = w.resetSlot(&w.Data[j]); err != nil {
			return err
		}
	}
	return nil
}

func (m *exactLUState) restoreExactUpdateBump(start, end uint32) error {
	local := m.updateWork
	if start < end {
		if uint64(end) >= uint64(len(m.rowEtaWork.Data)) {
			return ErrBounds
		}
		if err := local.copySlot(&local.Data[5], &m.rowEtaWork.Data[end]); err != nil {
			return err
		}
		row, err := m.U.AdjustRow(end)
		if err != nil {
			return err
		}
		if _, err = m.U.SetRowFromWorkVectorAndCleanNotAdjusted(row, m.rowEtaWork, m.Settings); err != nil {
			return err
		}
	} else {
		value, err := m.U.Get(end, end)
		if err != nil {
			return err
		}
		if err = local.copySlot(&local.Data[5], value); err != nil {
			return err
		}
	}
	zero, err := m.A.manager.IsZero(&local.Data[5])
	if err != nil {
		return err
	}
	if zero {
		m.status = LUDegenerated
		return nil
	}
	// calculate_Lwave_Pwave_for_last_row has its own value parameter.
	if err = local.copySlot(&local.Data[6], &local.Data[5]); err != nil {
		return err
	}
	m.pendingDiagonal, err = NewRationalDiagonalMatrix(m.A.manager, end, &local.Data[6])
	if err != nil {
		return err
	}
	return nil
}

func (m *exactLUState) clearDegeneratedUpdate() error {
	if err := m.rowEtaWork.ClearAll(); err != nil {
		return err
	}
	if m.pendingRowEta != nil {
		if err := m.pendingRowEta.Close(); err != nil {
			return err
		}
		m.pendingRowEta = nil
	}
	return nil
}

func (m *RationalLU) ReplaceColumn(pivotCheck *numeral.Rational, w *IndexedRationalVector, leaving uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finishUpdate(&err)
	if err = m.beginUpdate(pivotCheck, w); err != nil {
		return err
	}
	start, end, err := m.transformUpdateColumn(w, leaving)
	if err != nil {
		return err
	}
	m.RWave.Init(m.Dim)
	if err = m.calculateExactRWave(start, end, &m.RWave.Permutation); err != nil {
		return err
	}
	if err = m.makeExactUpdateRowEta(start, end); err != nil {
		return err
	}
	if m.status == LUDegenerated {
		return m.clearDegeneratedUpdate()
	}
	if err = m.Q.MultiplyRight(&m.RWave.Permutation); err != nil {
		return err
	}
	if err = m.R.MultiplyReverseLeft(&m.RWave.Permutation); err != nil {
		return err
	}
	if m.pendingRowEta != nil {
		if err = m.pendingRowEta.Conjugate(m.q); err != nil {
			return err
		}
		m.Tail = append(m.Tail, m.pendingRowEta)
		m.pendingRowEta = nil
	}
	if err = m.restoreExactUpdateBump(start, end); err != nil {
		return err
	}
	if m.status == LUDegenerated {
		return nil
	}
	tail := m.pendingDiagonal
	m.Tail = append(m.Tail, tail)
	m.pendingDiagonal = nil
	if err = m.U.DivideRow(end, &m.updateWork.Data[6], m.Settings); err != nil {
		return err
	}
	return tail.Conjugate(m.q)
}

func (m *PairLU) ReplaceColumn(pivotCheck *numeral.Rational, w *IndexedRationalVector, leaving uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finishUpdate(&err)
	if err = m.beginUpdate(pivotCheck, w); err != nil {
		return err
	}
	start, end, err := m.transformUpdateColumn(w, leaving)
	if err != nil {
		return err
	}
	m.RWave.Init(m.Dim)
	if err = m.calculateExactRWave(start, end, &m.RWave.Permutation); err != nil {
		return err
	}
	if err = m.makeExactUpdateRowEta(start, end); err != nil {
		return err
	}
	if m.status == LUDegenerated {
		return m.clearDegeneratedUpdate()
	}
	if err = m.Q.MultiplyRight(&m.RWave.Permutation); err != nil {
		return err
	}
	if err = m.R.MultiplyReverseLeft(&m.RWave.Permutation); err != nil {
		return err
	}
	if m.pendingRowEta != nil {
		if err = m.pendingRowEta.Conjugate(m.q); err != nil {
			return err
		}
		m.Tail = append(m.Tail, &PairRowEtaMatrix{RationalRowEtaMatrix: m.pendingRowEta})
		m.pendingRowEta = nil
	}
	if err = m.restoreExactUpdateBump(start, end); err != nil {
		return err
	}
	if m.status == LUDegenerated {
		return nil
	}
	tail := m.pendingDiagonal
	m.Tail = append(m.Tail, &PairDiagonalMatrix{RationalDiagonalMatrix: tail})
	m.pendingDiagonal = nil
	if err = m.U.DivideRow(end, &m.updateWork.Data[6], m.Settings); err != nil {
		return err
	}
	return tail.Conjugate(m.q)
}
