package lp

// Source: src/util/lp/lp_solver.hpp:68-77; retained mpq/mpq outer consumers
// in src/test/lp.cpp:1282-1303. Lookup dispatches to the concrete value getter.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func (m *RationalPrimalSimplex) GetColumnValueByName(name string, out *numeral.Rational) (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	column, ok := m.NamesToColumns[name]
	if !ok {
		return z3exception.NewDefaultException(fmt.Sprintf("get_column_value_by_name %s", name))
	}
	return m.GetColumnValue(column, out)
}

func (m *RationalDualSimplex) GetColumnValueByName(name string, out *numeral.Rational) (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	column, ok := m.NamesToColumns[name]
	if !ok {
		return z3exception.NewDefaultException(fmt.Sprintf("get_column_value_by_name %s", name))
	}
	return m.GetColumnValue(column, out)
}
