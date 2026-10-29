package lp

// Source: src/util/lp/lu.hpp:193-229,260-276,503-523.

// PrepareEntering builds Q^-1*L_k*...*L_0*A[:,entering], the column
// consumed by the source's triangular solve and Suhl update algorithms.
func (m *FloatLU) PrepareEntering(entering uint32, w *IndexedFloatVector) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finish(&err)
	if err = w.Clear(); err != nil {
		return err
	}
	if err = m.A.CopyColumnToIndexedVector(entering, w); err != nil {
		return err
	}
	for _, tail := range m.Tail {
		if err = tail.ApplyLeftToT(w, m.Settings); err != nil {
			return err
		}
	}
	return m.Q.ApplyReverseLeftIndexed(w)
}

func (m *FloatLU) SolveBd(entering uint32, d, w *IndexedFloatVector) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finish(&err)
	if err = m.PrepareEntering(entering, w); err != nil {
		return err
	}
	if luSparseColumnRoute(w.Size(), d.DataSize(), 120) {
		if err = d.Assign(w); err != nil {
			return err
		}
		return m.SolveByForTIndexedOnly(d, m.Settings)
	}
	d.Data = append(d.Data[:0], w.Data...)
	d.Index = d.Index[:0]
	return m.SolveByWhenReadyForT(d.Data, &d.Index)
}
