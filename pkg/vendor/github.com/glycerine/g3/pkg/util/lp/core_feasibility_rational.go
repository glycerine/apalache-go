package lp

// Source: src/util/lp/lp_core_solver_base.h:674-680; lp_core_solver_base.hpp:516-540.

import (
	"fmt"
	"os"
)

func (m *RationalCoreSolverBase) UpdateColumnInInfSet(j uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	feasible, e := m.ColumnIsFeasible(j)
	if e != nil {
		return e
	}
	if feasible {
		m.InfSet.Erase(j)
		return nil
	}
	return m.InfSet.Insert(j)
}
func (m *RationalCoreSolverBase) CalcCurrentXIsFeasibleIncludeNonBasis() (feasible bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	for j := m.A.ColumnCount(); j > 0; {
		j--
		ok, e := m.ColumnIsFeasible(j)
		if e != nil {
			return false, e
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}
func (m *RationalCoreSolverBase) InfSetIsCorrect() (ok bool, err error) {
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
