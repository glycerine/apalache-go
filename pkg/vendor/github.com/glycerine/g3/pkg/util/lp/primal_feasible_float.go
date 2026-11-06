package lp

// Source: src/util/lp/lp_core_solver_base.h:541-548,568-598;
// lp_primal_core_solver.hpp:1043-1048. Retained double feasibility entry point.

import (
	"fmt"
	"io"
	"os"
)

// PrintColumnInfo preserves the source split: column/bounds go to out, while
// basis heading and value always go to standard output.
func (m *FloatCoreSolverBase) PrintColumnInfo(j uint32, out io.Writer) error {
	if out == nil || m.Names == nil || uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return ErrBounds
	}
	typ := (*m.ColumnTypes)[j]
	name := m.Names.ColumnName(j)
	typeName, err := ColumnTypeString(typ)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "column_index = %d, name = %s type = %s\n", j, name, typeName); err != nil {
		return err
	}
	switch typ {
	case ColumnFixed, ColumnBoxed:
		if uint64(j) >= uint64(len(*m.LowBounds)) || uint64(j) >= uint64(len(*m.UpperBounds)) {
			return ErrBounds
		}
		_, err = fmt.Fprintf(out, "(%s, %s)\n", floatLPNumberText((*m.LowBounds)[j]), floatLPNumberText((*m.UpperBounds)[j]))
	case ColumnLowBound:
		if uint64(j) >= uint64(len(*m.LowBounds)) {
			return ErrBounds
		}
		_, err = fmt.Fprintln(out, floatLPNumberText((*m.LowBounds)[j]))
	case ColumnUpperBound:
		if uint64(j) >= uint64(len(*m.UpperBounds)) {
			return ErrBounds
		}
		_, err = fmt.Fprintln(out, floatLPNumberText((*m.UpperBounds)[j]))
	case ColumnFree:
	default:
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}
	if uint64(j) >= uint64(len(*m.Heading)) {
		return ErrBounds
	}
	if _, err := fmt.Fprintf(os.Stdout, "basis heading = %d\n", (*m.Heading)[j]); err != nil {
		return err
	}
	if uint64(j) >= uint64(len(*m.X)) {
		return ErrBounds
	}
	_, err = fmt.Fprintf(os.Stdout, "x = %s\n", floatLPNumberText((*m.X)[j]))
	return err
}

// NonBasicColumnsAreSetCorrectly checks feasibility, rather than requiring
// nonbasic values to equal bounds. This distinction follows the source body.
func (m *FloatCoreSolverBase) NonBasicColumnsAreSetCorrectly() (bool, error) {
	for _, j := range *m.NBasis {
		ok, err := m.ColumnIsFeasible(j)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, m.PrintColumnInfo(j, os.Stdout)
		}
	}
	return true, nil
}

func (m *FloatPrimalCoreSolver) FindFeasibleSolution() error {
	m.LookForFeasibleSolutionOnly = true
	if sourceDebugEnabled {
		ok, err := m.NonBasicColumnsAreSetCorrectly()
		if err != nil {
			return err
		}
		if !ok {
			return ErrAssertion
		}
	}
	m.Status = StatusUnknown
	_, err := m.Solve()
	return err
}
