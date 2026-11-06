package lp

// Source: src/util/lp/lar_solver.h:752-767; lp_core_solver_base.hpp:526-541.

import (
	"fmt"
	"os"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *LARSolver) RowIsCorrect(row uint32) (ok bool, err error) {
	if err = s.ready(); err != nil {
		return false, err
	}
	defer func() { s.stopped = err != nil }()
	if uint64(row) >= uint64(len(s.Core.RA.Rows)) {
		return false, ErrBounds
	}
	s.work = NewIndexedPairVector(s.manager, 2)
	s.rationalWork = NewIndexedRationalVector(s.manager, 3)
	before, e := s.calls.begin(s.manager)
	if e != nil {
		return false, e
	}
	ok, err = func() (bool, error) {
		sum, product := &s.work.Data[0], &s.work.Data[1]
		tmp, px, py := &s.rationalWork.Data[0], &s.rationalWork.Data[1], &s.rationalWork.Data[2]
		for k := range s.Core.RA.Rows[row] {
			cell := &s.Core.RA.Rows[row][k]
			if uint64(cell.Column) >= uint64(len(s.Core.RX.Data)) {
				return false, ErrBounds
			}
			x := &s.Core.RX.Data[cell.Column]
			if e := etaRationalProduct(s.manager, tmp, px, &cell.Value, &x.X); e != nil {
				return false, e
			}
			if e := etaRationalProduct(s.manager, tmp, py, &cell.Value, &x.Y); e != nil {
				return false, e
			}
			if e := s.manager.Copy(&product.X, px); e != nil {
				return false, e
			}
			if e := s.manager.Copy(&product.Y, py); e != nil {
				return false, e
			}
			if e := sum.Add(s.manager, sum, product); e != nil {
				return false, e
			}
			if e := product.Del(s.manager); e != nil {
				return false, e
			}
			if e := s.manager.Del(py); e != nil {
				return false, e
			}
			*py = numeral.NewRational()
			if e := s.manager.Del(px); e != nil {
				return false, e
			}
			*px = numeral.NewRational()
		}
		return sum.IsZero(s.manager)
	}()
	if err = s.calls.after(s.manager, before, err); err != nil {
		return false, err
	}
	if err = s.rationalWork.Close(); err != nil {
		return false, err
	}
	s.rationalWork = nil
	if err = s.work.Close(); err != nil {
		return false, err
	}
	s.work = nil
	return ok, nil
}

func (s *LARSolver) AxIsCorrect() (ok bool, err error) {
	if err = s.ready(); err != nil {
		return false, err
	}
	defer func() { s.stopped = err != nil }()
	for row := uint32(0); row < s.Core.RA.RowCount(); row++ {
		ok, err = s.RowIsCorrect(row)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

func (m *PairCoreSolverBase) InfSetIsCorrect() (ok bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	for j := m.A.ColumnCount(); j > 0; {
		j--
		belongs := m.InfSet.Contains(j)
		feasible, e := m.ColumnIsFeasible(j)
		if e != nil {
			return false, e
		}
		if feasible == belongs {
			if err = m.PrintColumnInfo(j, os.Stdout); err != nil {
				return false, err
			}
			member := 0
			if belongs {
				member = 1
			}
			if _, err = fmt.Fprintf(os.Stdout, "belongs_to_set = %d\n", member); err != nil {
				return false, err
			}
			text := "inf"
			if feasible {
				text = "feas"
			}
			_, err = fmt.Fprintln(os.Stdout, text)
			return false, err
		}
	}
	return true, nil
}
