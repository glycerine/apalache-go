package lp

// Source: src/util/lp/lp_core_solver_base.hpp:225-240,473-477; scalar mpq/mpq.

import (
	"fmt"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (m *RationalCoreSolverBase) buildRowResidual(i uint32) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	w := m.residualWork
	if err = etaRationalDifference(m.manager, &w.Data[2], &w.Data[1], &(*m.B)[i], &w.Data[0]); err != nil {
		return err
	}
	if err = m.manager.Del(&w.Data[0]); err != nil {
		return err
	}
	w.Data[0] = numeral.NewRational()
	return nil
}

// Preserve the second row product's evaluation inside the output expression.
func (m *RationalCoreSolverBase) printRowResidual(i uint32, out io.Writer) (err error) {
	if _, err = fmt.Fprint(out, "precise x is off ("); err != nil {
		return err
	}
	if _, err = fmt.Fprintf(out, "m_b[%d] = ", i); err != nil {
		return err
	}
	rhs, e := m.manager.String(&(*m.B)[i])
	if e != nil {
		return e
	}
	if _, err = fmt.Fprint(out, rhs, " "); err != nil {
		return err
	}
	if _, err = fmt.Fprint(out, "left side = "); err != nil {
		return err
	}
	dot := &m.residualWork.Data[0]
	if err = m.A.DotRow(i, *m.X, dot); err != nil {
		return err
	}
	text, e := m.manager.String(dot)
	if e != nil {
		return e
	}
	if _, err = fmt.Fprint(out, text, " "); err != nil {
		return err
	}
	if err = m.manager.Del(dot); err != nil {
		return err
	}
	*dot = numeral.NewRational()
	if _, err = fmt.Fprint(out, "delta = "); err != nil {
		return err
	}
	delta, e := m.manager.String(&m.residualWork.Data[1])
	if e != nil {
		return e
	}
	if _, err = fmt.Fprint(out, delta, " "); err != nil {
		return err
	}
	if _, err = fmt.Fprintf(out, "iters = %d)\n", m.TotalIterations); err != nil {
		return err
	}
	if f, ok := out.(interface{ Flush() error }); ok {
		return f.Flush()
	}
	return nil
}

func (m *RationalCoreSolverBase) AMultXIsOff() (off bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	return m.aMultXIsOffValue()
}
func (m *RationalCoreSolverBase) aMultXIsOffValue() (off bool, err error) {
	if sourceDebugEnabled && len(*m.X) != int(m.A.ColumnCount()) {
		return false, ErrAssertion
	}
	for i := uint32(0); i < m.A.RowCount(); i++ {
		if uint64(i) >= uint64(len(*m.B)) {
			return false, ErrBounds
		}
		m.residualWork = NewIndexedRationalVector(m.manager, 3)
		if err = m.A.DotRow(i, *m.X, &m.residualWork.Data[0]); err != nil {
			return false, err
		}
		if err = m.buildRowResidual(i); err != nil {
			return false, err
		}
		zero, e := m.manager.IsZero(&m.residualWork.Data[1])
		if e != nil {
			return false, e
		}
		if !zero {
			if err = m.printRowResidual(i, os.Stdout); err != nil {
				return false, err
			}
		}
		if err = m.residualWork.Close(); err != nil {
			return false, err
		}
		m.residualWork = nil
		if !zero {
			return true, nil
		}
	}
	return false, nil
}

func (m *RationalCoreSolverBase) FindXBySolving() (ok bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	if err = m.solveAxEqualsBValue(); err != nil {
		return false, err
	}
	off, e := m.aMultXIsOffValue()
	if e != nil {
		return false, e
	}
	return !off, nil
}
