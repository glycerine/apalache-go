package lp

// Source: src/util/lp/lu.hpp:427-438,465-469, retained precise branches.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *exactLUState) rowRefinementInput(y *IndexedRationalVector) error {
	if y.manager != m.A.manager {
		return ErrExactVectorOwner
	}
	if y.calls.pending() {
		return ErrExactVectorPending
	}
	return nil
}

func (m *RationalLU) SolveYBWithErrorCheck(y []numeral.Rational, _ []uint32) error {
	return m.SolveYB(y)
}

func (m *PairLU) SolveYBWithErrorCheck(y []numeral.Rational, _ []uint32) error {
	return m.SolveYB(y)
}

func (m *RationalLU) SolveYBWithErrorCheckIndexed(y *IndexedRationalVector, _ []int32, _ []uint32, _ *Settings) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finish(&err)
	if err = m.rowRefinementInput(y); err != nil {
		return err
	}
	if luSparseColumnRoute(y.Size(), m.A.ColumnCount(), 30) {
		return m.SolveYBIndexed(y)
	}
	if err = m.SolveYB(y.Data); err != nil {
		return err
	}
	return y.RestoreIndex()
}

func (m *PairLU) SolveYBWithErrorCheckIndexed(y *IndexedRationalVector, _ []int32, _ []uint32, _ *Settings) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finish(&err)
	if err = m.rowRefinementInput(y); err != nil {
		return err
	}
	if luSparseColumnRoute(y.Size(), m.A.ColumnCount(), 30) {
		return m.SolveYBIndexed(y)
	}
	if err = m.SolveYB(y.Data); err != nil {
		return err
	}
	return y.RestoreIndex()
}
