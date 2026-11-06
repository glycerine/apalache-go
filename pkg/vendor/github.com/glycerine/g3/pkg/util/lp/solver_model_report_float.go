package lp

// Source: src/util/lp/lp_solver.h:259-264; retained floating primal caller
// src/shell/lp_frontend.cpp:92.

import (
	"io"
	"sort"
)

// PrintModel reports the original objective and named external column values.
// The source name map is unordered; native string sorting makes its report
// reproducible without introducing an insertion-order requirement.
func (m *FloatPrimalSimplex) PrintModel(out io.Writer) error {
	if err := writeConstraintText(out, "objective = "); err != nil {
		return err
	}
	cost, err := m.GetCurrentCost()
	if err != nil {
		return err
	}
	if err = writeDriverLine(out, floatLPNumberText(cost)); err != nil {
		return err
	}
	if err = writeConstraintText(out, "column values\n"); err != nil {
		return err
	}
	names := make([]string, 0, len(m.NamesToColumns))
	for name := range m.NamesToColumns {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err = writeConstraintText(out, name+" = "); err != nil {
			return err
		}
		value, e := m.GetColumnValue(m.NamesToColumns[name])
		if e != nil {
			return e
		}
		if err = writeDriverLine(out, floatLPNumberText(value)); err != nil {
			return err
		}
	}
	return nil
}
