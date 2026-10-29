package lp

// Source: src/util/lp/lu.hpp:177-197,285-313,497-529.

// SolveBy solves B*y=rhs in place. The source's dense floating triangular
// refinement and final drop pass remain distinct from the tail transformations.
func (m *FloatLU) SolveBy(y []float64) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finish(&err)
	for _, tail := range m.Tail {
		if err = tail.ApplyLeft(y); err != nil {
			return err
		}
	}
	if err = m.Q.ApplyReverseLeftToX(y); err != nil {
		return err
	}
	return m.SolveByWhenReadyForX(y)
}

func (m *FloatLU) SolveYB(y []float64) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finish(&err)
	if err = m.R.ApplyReverseRightToT(y); err != nil {
		return err
	}
	if err = m.U.SolveYU(y); err != nil {
		return err
	}
	if err = m.Q.ApplyReverseRightToT(y); err != nil {
		return err
	}
	for i := len(m.Tail); i > 0; {
		i--
		if err = m.Tail[i].ApplyRight(y); err != nil {
			return err
		}
	}
	return nil
}
func (m *FloatLU) SolveYBIndexed(y *IndexedFloatVector) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finish(&err)
	if err = m.R.ApplyReverseRightIndexed(y); err != nil {
		return err
	}
	if err = m.U.SolveYUIndexed(y, m.Settings); err != nil {
		return err
	}
	if err = m.Q.ApplyReverseRightIndexed(y); err != nil {
		return err
	}
	for i := len(m.Tail); i > 0; {
		i--
		if err = m.Tail[i].ApplyRightIndexed(y); err != nil {
			return err
		}
	}
	return nil
}
