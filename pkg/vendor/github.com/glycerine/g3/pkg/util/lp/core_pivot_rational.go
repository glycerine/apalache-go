package lp

// Source: src/util/lp/lp_core_solver_base.hpp:301-334, mpq/mpq.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *RationalCoreSolverBase) CalculatePivotRowOfBInverse(row uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if sourceDebugEnabled {
		if m.Settings == nil {
			return ErrBounds
		}
		if m.Settings.UseTableau() {
			return ErrAssertion
		}
	}
	// indexed_vector::is_OK is the source's true stub.
	if err = m.PivotRowB1.Clear(); err != nil {
		return err
	}
	one := numeral.NewRational()
	one.Num = numeral.NewInt(1)
	if err = m.PivotRowB1.SetValue(&one, row); err != nil {
		return err
	}
	if m.Factorization == nil {
		return ErrBounds
	}
	// The LU, not this core, owns the stopped solve and refinement calls.
	return m.Factorization.SolveYBWithErrorCheckIndexed(m.PivotRowB1, *m.Heading, *m.Basis, m.Settings)
}

func (m *RationalCoreSolverBase) CalculatePivotRowWhenInverseReady(row uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if err = m.PivotRow.Clear(); err != nil {
		return err
	}
	m.pivotWork = NewIndexedRationalVector(m.manager, 2)
	for _, i := range m.PivotRowB1.Index {
		if uint64(i) >= uint64(len(m.PivotRowB1.Data)) {
			return ErrBounds
		}
		v := &m.PivotRowB1.Data[i]
		zero, e := m.manager.IsZero(v)
		if e != nil {
			return e
		}
		if zero {
			continue
		}
		if uint64(i) >= uint64(len(m.A.Rows)) {
			return ErrBounds
		}
		for k := range m.A.Rows[i] {
			c := &m.A.Rows[i][k]
			if uint64(c.Column) >= uint64(len(*m.Heading)) {
				return ErrBounds
			}
			if (*m.Heading)[c.Column] >= 0 {
				continue
			}
			before, e := m.calls.begin(m.manager)
			if e != nil {
				return e
			}
			tmp, product := &m.pivotWork.Data[0], &m.pivotWork.Data[1]
			e = etaRationalProduct(m.manager, tmp, product, &c.Value, v)
			if e = m.calls.after(m.manager, before, e); e != nil {
				return e
			}
			if e = m.PivotRow.AddWithDropTolerance(c.Column, product); e != nil {
				return e
			}
			// Finish the return-value temporary only after the child's add.
			if e = m.manager.Del(product); e != nil {
				return e
			}
			*product = numeral.NewRational()
		}
	}
	if uint64(row) >= uint64(len(m.RowsNZ)) {
		return ErrBounds
	}
	m.RowsNZ[row] = m.PivotRow.Size()
	if err = m.pivotWork.Close(); err != nil {
		return err
	}
	m.pivotWork = nil
	return nil
}
