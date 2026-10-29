package lp

// Source: src/util/lp/lu.hpp:175-229, retained double/double prepared solves.

// SolveByWhenReadyForX consumes Q^-1*L*rhs, without applying the tails again.
func (m *FloatLU) SolveByWhenReadyForX(y []float64) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finish(&err)
	if err = m.U.DoubleSolveUY(y); err != nil {
		return err
	}
	if err = m.R.ApplyReverseLeftToX(y); err != nil {
		return err
	}
	for i := m.Dim; i > 0; {
		i--
		if uint64(i) >= uint64(len(y)) {
			return ErrBounds
		}
		if y[i] != 0 && m.Settings.FloatDropSmall(y[i]) {
			y[i] = 0
		}
	}
	return nil
}

// SolveByWhenReadyForT appends descending nonzero solution indices. Existing
// entries are retained, matching the source's separate vector<unsigned> output.
func (m *FloatLU) SolveByWhenReadyForT(y []float64, index *[]uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finish(&err)
	if err = m.U.DoubleSolveUY(y); err != nil {
		return err
	}
	if err = m.R.ApplyReverseLeftToT(y); err != nil {
		return err
	}
	for i := m.Dim; i > 0; {
		i--
		if uint64(i) >= uint64(len(y)) {
			return ErrBounds
		}
		if y[i] == 0 {
			continue
		}
		if m.Settings.FloatDropSmall(y[i]) {
			y[i] = 0
		} else {
			*index = append(*index, i)
		}
	}
	return nil
}

// SolveByForTIndexedOnly is the triangular stage used by entering columns and
// the dual solver's DSE_FTran. Floating source code uses member settings here.
func (m *FloatLU) SolveByForTIndexedOnly(y *IndexedFloatVector, _ *Settings) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finish(&err)
	if err = m.U.DoubleSolveUYIndexed(y, m.Settings); err != nil {
		return err
	}
	return m.R.ApplyReverseLeftIndexed(y)
}
