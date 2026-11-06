package lp

// Source: src/util/lp/lp_core_solver_base.h:210-243,568-598,689-697;
// lp_core_solver_base.hpp:981-1044; retained mpq/impq diagnostics.

import (
	"fmt"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// PrintColumnInfo retains the source split stream: heading/value always go to stdout.
func (m *PairCoreSolverBase) PrintColumnInfo(j uint32, out io.Writer) error {
	if out == nil || m.Names == nil || uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return ErrBounds
	}
	typ := (*m.ColumnTypes)[j]
	name := m.Names.ColumnName(j)
	text, err := ColumnTypeString(typ)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(out, "column_index = %d, name = %s type = %s\n", j, name, text); err != nil {
		return err
	}
	switch typ {
	case ColumnFixed, ColumnBoxed:
		if uint64(j) >= uint64(len(*m.LowBounds)) || uint64(j) >= uint64(len(*m.UpperBounds)) {
			return ErrBounds
		}
		low, e := (*m.LowBounds)[j].String(m.manager)
		if e != nil {
			return e
		}
		upper, e := (*m.UpperBounds)[j].String(m.manager)
		if e != nil {
			return e
		}
		_, err = fmt.Fprintf(out, "(%s, %s)\n", low, upper)
	case ColumnLowBound, ColumnUpperBound:
		bounds := m.LowBounds
		if typ == ColumnUpperBound {
			bounds = m.UpperBounds
		}
		if uint64(j) >= uint64(len(*bounds)) {
			return ErrBounds
		}
		bound, e := (*bounds)[j].String(m.manager)
		if e != nil {
			return e
		}
		_, err = fmt.Fprintln(out, bound)
	case ColumnFree:
	default:
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}
	if uint64(j) >= uint64(len(*m.Heading)) {
		return ErrBounds
	}
	if _, err = fmt.Fprintf(os.Stdout, "basis heading = %d\n", (*m.Heading)[j]); err != nil {
		return err
	}
	if uint64(j) >= uint64(len(*m.X)) {
		return ErrBounds
	}
	value, err := (*m.X)[j].String(m.manager)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(os.Stdout, "x = %s\n", value)
	return err
}

func (m *PairCoreSolverBase) CostsOnNBasisAreZeros() (ok bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	// All source checks in this helper are assertions, including zero tests.
	if !sourceDebugEnabled {
		return true, nil
	}
	ok, err = m.BasisHeadingIsCorrect()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, ErrAssertion
	}
	for j := uint32(0); j < m.A.ColumnCount(); j++ {
		if uint64(j) >= uint64(len(*m.Heading)) {
			return false, ErrBounds
		}
		if (*m.Heading)[j] >= 0 {
			continue
		}
		if uint64(j) >= uint64(len(*m.Costs)) {
			return false, ErrBounds
		}
		zero, e := m.manager.IsZero(&(*m.Costs)[j])
		if e != nil {
			return false, e
		}
		if !zero {
			return false, ErrAssertion
		}
	}
	return true, nil
}

func (m *PairCoreSolverBase) InfeasibilityCostIsCorrectForColumn(j uint32) (ok bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	if uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return false, ErrBounds
	}
	kind := (*m.ColumnTypes)[j]
	if kind != ColumnFree && kind != ColumnFixed && kind != ColumnBoxed && kind != ColumnLowBound && kind != ColumnUpperBound {
		// Unlike primal cost construction, unknown native types return true here.
		return true, sourceAssertionFailure()
	}
	if uint64(j) >= uint64(len(*m.Costs)) {
		return false, ErrBounds
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return false, e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	var cost int32
	if kind != ColumnFree {
		if uint64(j) >= uint64(len(*m.X)) {
			return false, ErrBounds
		}
		x := &(*m.X)[j]
		above := false
		if kind == ColumnFixed || kind == ColumnBoxed || kind == ColumnUpperBound {
			if uint64(j) >= uint64(len(*m.UpperBounds)) {
				return false, ErrBounds
			}
			cmp, e := x.Compare(m.manager, &(*m.UpperBounds)[j])
			if e != nil {
				return false, e
			}
			above = cmp > 0
		}
		if above {
			cost = 1
		} else if kind == ColumnFixed || kind == ColumnBoxed || kind == ColumnLowBound {
			if uint64(j) >= uint64(len(*m.LowBounds)) {
				return false, ErrBounds
			}
			cmp, e := x.Compare(m.manager, &(*m.LowBounds)[j])
			if e != nil {
				return false, e
			}
			if cmp < 0 {
				cost = -1
			}
		}
	}
	if !m.Settings.UseBreakpointsInFeasibilitySearch {
		cost = -cost
	}
	if cost == 0 {
		return m.manager.IsZero(&(*m.Costs)[j])
	}
	expected := numeral.NewRational()
	if err = m.manager.SetInt32(&expected, cost); err != nil {
		return false, err
	}
	// The expected source scalar is inline +/-1 and owns no arena payload.
	return m.manager.Equal(&(*m.Costs)[j], &expected)
}

func (m *PairCoreSolverBase) InfeasibilityCostsAreCorrect() (ok bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	if !m.UsingInfeasCosts {
		return true, nil
	}
	if sourceDebugEnabled {
		ok, err = m.CostsOnNBasisAreZeros()
		if err != nil {
			return false, err
		}
		if !ok {
			return false, ErrAssertion
		}
	}
	for _, j := range *m.Basis {
		ok, err = m.InfeasibilityCostIsCorrectForColumn(j)
		if err != nil {
			return false, err
		}
		if !ok {
			if _, err = fmt.Fprintln(os.Stdout, "infeasibility_cost_is_correct_for_column does not hold"); err != nil {
				return false, err
			}
			return false, m.PrintColumnInfo(j, os.Stdout)
		}
		if uint64(j) >= uint64(len(m.D.Data)) {
			return false, ErrBounds
		}
		zero, e := m.manager.IsZero(&m.D.Data[j])
		if e != nil {
			return false, e
		}
		if !zero {
			if _, err = fmt.Fprintln(os.Stdout, "m_d is not zero"); err != nil {
				return false, err
			}
			return false, m.PrintColumnInfo(j, os.Stdout)
		}
	}
	return true, nil
}

func (m *PairCoreSolverBase) ReducedCostsAreCorrectTableau() (ok bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	if m.Settings.SimplexStrategy == StrategyTableauRows {
		return true, nil
	}
	if sourceDebugEnabled {
		ok, err = m.A.IsCorrect(nil)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, ErrAssertion
		}
	}
	if m.UsingInfeasCosts {
		ok, err = m.InfeasibilityCostsAreCorrect()
		if err != nil {
			return false, err
		}
		if !ok {
			_, err = fmt.Fprintln(os.Stdout, "infeasibility_costs_are_correct() does not hold")
			return false, err
		}
	}
	for j := uint32(0); j < m.A.ColumnCount(); j++ {
		if uint64(j) >= uint64(len(*m.Heading)) || uint64(j) >= uint64(len(m.D.Data)) {
			return false, ErrBounds
		}
		if (*m.Heading)[j] >= 0 {
			zero, e := m.manager.IsZero(&m.D.Data[j])
			if e != nil {
				return false, e
			}
			if !zero {
				if _, err = fmt.Fprint(os.Stdout, "case a\n"); err != nil {
					return false, err
				}
				return false, m.PrintColumnInfo(j, os.Stdout)
			}
			continue
		}
		if uint64(j) >= uint64(len(*m.Costs)) {
			return false, ErrBounds
		}
		m.pivotWork = NewIndexedRationalVector(m.manager, 3)
		before, e := m.calls.begin(m.manager)
		if e != nil {
			return false, e
		}
		ok, err = func() (bool, error) {
			d, tmp, product := &m.pivotWork.Data[0], &m.pivotWork.Data[1], &m.pivotWork.Data[2]
			if e := m.manager.Copy(d, &(*m.Costs)[j]); e != nil {
				return false, e
			}
			for _, cell := range m.A.Columns[j] {
				if uint64(cell.Row) >= uint64(len(*m.Basis)) {
					return false, ErrBounds
				}
				basic := (*m.Basis)[cell.Row]
				if uint64(basic) >= uint64(len(*m.Costs)) {
					return false, ErrBounds
				}
				coefficient, e := m.A.ColumnValue(cell)
				if e != nil {
					return false, e
				}
				if e = etaRationalProduct(m.manager, tmp, product, &(*m.Costs)[basic], coefficient); e != nil {
					return false, e
				}
				if e = m.manager.Sub(d, product, d); e != nil {
					return false, e
				}
				if e = m.manager.Del(product); e != nil {
					return false, e
				}
				*product = numeral.NewRational()
			}
			return m.manager.Equal(&m.D.Data[j], d)
		}()
		if err = m.calls.after(m.manager, before, err); err != nil {
			return false, err
		}
		if err = m.pivotWork.Close(); err != nil {
			return false, err
		}
		m.pivotWork = nil
		if !ok {
			if _, err = fmt.Fprint(os.Stdout, "case b\n"); err != nil {
				return false, err
			}
			return false, m.PrintColumnInfo(j, os.Stdout)
		}
	}
	return true, nil
}
