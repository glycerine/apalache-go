package lp

// Source: src/util/lp/lu.hpp:175-229, retained precise prepared solves.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *RationalLU) SolveByWhenReadyForX(y []numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finish(&err)
	if err = m.U.SolveUY(y); err != nil {
		return err
	}
	if err = m.R.ApplyReverseLeftToX(y); err != nil {
		return err
	}
	return nil
}
func (m *RationalLU) SolveByWhenReadyForT(y []numeral.Rational, index *[]uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finish(&err)
	if err = m.U.SolveUY(y); err != nil {
		return err
	}
	if err = m.R.ApplyReverseLeftToT(y); err != nil {
		return err
	}
	return m.appendColumnIndex(y, index)
}

func (m *RationalLU) SolveByForTIndexedOnly(y *IndexedRationalVector, s *Settings) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finish(&err)
	var active []uint32
	if err = m.U.SolveUYIndexed(y, s, &active); err != nil {
		return err
	}
	return m.R.ApplyReverseLeftIndexed(y)
}

func (m *PairLU) SolveByWhenReadyForX(y []RationalPair) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finish(&err)
	if err = m.U.SolveUYPair(y); err != nil {
		return err
	}
	if err = m.R.ApplyReverseLeftToX(y); err != nil {
		return err
	}
	return nil
}
func (m *PairLU) SolveByWhenReadyForT(y []numeral.Rational, index *[]uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finish(&err)
	if err = m.U.SolveUY(y); err != nil {
		return err
	}
	if err = m.R.ApplyReverseLeftToT(y); err != nil {
		return err
	}
	return m.appendColumnIndex(y, index)
}

func (m *PairLU) SolveByForTIndexedOnly(y *IndexedRationalVector, s *Settings) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finish(&err)
	var active []uint32
	if err = m.U.SolveUYIndexed(y, s, &active); err != nil {
		return err
	}
	return m.R.ApplyReverseLeftIndexed(y)
}
