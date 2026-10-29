package lp

// Source: src/util/lp/sparse_matrix.hpp:467-493,679-705, live mpq right sides.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *RationalSparseMatrix) indexedSolveInput(y *IndexedRationalVector) error {
	if err := m.ready(); err != nil {
		return err
	}
	if y.manager != m.manager {
		return ErrExactVectorOwner
	}
	if y.calls.pending() {
		return ErrExactVectorPending
	}
	return nil
}

// SolveYUIndexed borrows the exact indexed destination. The matrix owns actual
// anchor-first products and subtraction calls; caller owns partial data/index.
// Close a stopped matrix before the destination. Exact drop policy tests zero.
func (m *RationalSparseMatrix) SolveYUIndexed(y *IndexedRationalVector, _ *Settings) (err error) {
	if err = m.indexedSolveInput(y); err != nil {
		return err
	}
	b, err := m.beginWork(2)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	var active []uint32
	if err = m.extendActiveRows(y.Index, &active); err != nil {
		return err
	}
	for k := len(active); k > 0; {
		k--
		j := active[k]
		if uint64(j) >= uint64(len(y.Data)) {
			return ErrBounds
		}
		col, e := m.AdjustColumn(j)
		if e != nil {
			return e
		}
		if uint64(col) >= uint64(len(m.Columns)) {
			return ErrBounds
		}
		for k := range m.Columns[col].Values {
			v := &m.Columns[col].Values[k]
			i, e := m.AdjustRowInverse(v.Index)
			if e != nil {
				return e
			}
			if i == j {
				continue
			}
			if uint64(i) >= uint64(len(y.Data)) {
				return ErrBounds
			}
			product := &m.work.Data[1]
			if err = etaRationalProduct(m.manager, &m.work.Data[0], product, &y.Data[i], &v.Value); err != nil {
				return err
			}
			if err = m.manager.Sub(&y.Data[j], product, &y.Data[j]); err != nil {
				return err
			}
			if err = m.manager.Del(product); err != nil {
				return err
			}
			*product = numeral.NewRational()
		}
	}
	y.Index = y.Index[:0]
	for _, j := range active {
		if uint64(j) >= uint64(len(y.Data)) {
			return ErrBounds
		}
		zero, e := m.manager.IsZero(&y.Data[j])
		if e != nil {
			return e
		}
		if !zero {
			y.Index = append(y.Index, j)
		}
	}
	return nil
}

// SolveUYIndexed uses the retained column graph postorder, appending to active.
// Numerical failures leave caller data and its old index until rebuilding starts.
func (m *RationalSparseMatrix) SolveUYIndexed(y *IndexedRationalVector, _ *Settings, active *[]uint32) (err error) {
	if err = m.indexedSolveInput(y); err != nil {
		return err
	}
	b, err := m.beginWork(2)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	if err = m.createGraph(y.Index, active); err != nil {
		return err
	}
	for k := len(*active); k > 0; {
		k--
		j := (*active)[k]
		if uint64(j) >= uint64(len(y.Data)) {
			return ErrBounds
		}
		zero, e := m.manager.IsZero(&y.Data[j])
		if e != nil {
			return e
		}
		if zero {
			continue
		}
		col, e := m.AdjustColumn(j)
		if e != nil {
			return e
		}
		if uint64(col) >= uint64(len(m.Columns)) {
			return ErrBounds
		}
		for k := range m.Columns[col].Values {
			v := &m.Columns[col].Values[k]
			i, e := m.AdjustRowInverse(v.Index)
			if e != nil {
				return e
			}
			if i == j {
				continue
			}
			if uint64(i) >= uint64(len(y.Data)) {
				return ErrBounds
			}
			product := &m.work.Data[1]
			if err = etaRationalProduct(m.manager, &m.work.Data[0], product, &v.Value, &y.Data[j]); err != nil {
				return err
			}
			if err = m.manager.Sub(&y.Data[i], product, &y.Data[i]); err != nil {
				return err
			}
			if err = m.manager.Del(product); err != nil {
				return err
			}
			*product = numeral.NewRational()
		}
	}
	y.Index = y.Index[:0]
	for _, j := range *active {
		if uint64(j) >= uint64(len(y.Data)) {
			return ErrBounds
		}
		zero, e := m.manager.IsZero(&y.Data[j])
		if e != nil {
			return e
		}
		if !zero {
			y.Index = append(y.Index, j)
		}
	}
	return nil
}
