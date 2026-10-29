package lp

// Source: src/util/lp/lu.hpp:177-190,285-313,497-529.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *RationalLU) SolveBy(y []numeral.Rational) (err error) {
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

func (m *RationalLU) SolveYB(y []numeral.Rational) (err error) {
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
func (m *RationalLU) SolveYBIndexed(y *IndexedRationalVector) (err error) {
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
func (m *PairLU) SolveBy(y []RationalPair) (err error) {
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

func (m *PairLU) SolveYB(y []numeral.Rational) (err error) {
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
func (m *PairLU) SolveYBIndexed(y *IndexedRationalVector) (err error) {
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
