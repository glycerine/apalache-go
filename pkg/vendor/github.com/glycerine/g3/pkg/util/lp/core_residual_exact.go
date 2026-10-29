package lp

// Source: src/util/lp/lp_core_solver_base.hpp:225-240, precise branch.

import (
	"fmt"
	"io"
	"os"
)

func (m *PairCoreSolverBase) buildRowResidual(i uint32) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.residualScalars = NewIndexedRationalVector(m.manager, 3)
	tmp, x, y := &m.residualScalars.Data[0], &m.residualScalars.Data[1], &m.residualScalars.Data[2]
	dot, delta := &m.residualWork.Data[0], &m.residualWork.Data[1]
	if err = etaRationalDifference(m.manager, tmp, x, &(*m.B)[i].X, &dot.X); err != nil {
		return err
	}
	if err = etaRationalDifference(m.manager, tmp, y, &(*m.B)[i].Y, &dot.Y); err != nil {
		return err
	}
	if err = m.manager.Copy(&delta.X, x); err != nil {
		return err
	}
	if err = m.manager.Copy(&delta.Y, y); err != nil {
		return err
	}
	if err = m.residualScalars.Close(); err != nil {
		return err
	}
	m.residualScalars = nil
	return dot.Del(m.manager)
}

// Retain source stdout chunks and the second matrix product's evaluation point.
func (m *PairCoreSolverBase) printRowResidual(i uint32, out io.Writer) (err error) {
	if _, err = fmt.Fprint(out, "precise x is off ("); err != nil {
		return err
	}
	if _, err = fmt.Fprintf(out, "m_b[%d] = ", i); err != nil {
		return err
	}
	rhs, e := (*m.B)[i].String(m.manager)
	if e != nil {
		return e
	}
	if _, err = fmt.Fprint(out, rhs, " "); err != nil {
		return err
	}
	if _, err = fmt.Fprint(out, "left side = "); err != nil {
		return err
	}
	// The source recomputes this row inside the output expression.
	dot := &m.residualWork.Data[0]
	if err = m.A.DotPairRow(i, *m.X, dot); err != nil {
		return err
	}
	text, e := dot.String(m.manager)
	if e != nil {
		return e
	}
	if _, err = fmt.Fprint(out, text, " "); err != nil {
		return err
	}
	if err = dot.Del(m.manager); err != nil {
		return err
	}
	if _, err = fmt.Fprint(out, "delta = "); err != nil {
		return err
	}
	delta, e := m.residualWork.Data[1].String(m.manager)
	if e != nil {
		return e
	}
	if _, err = fmt.Fprint(out, delta, " "); err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "iters = %d)\n", m.TotalIterations)
	return err
}

func (m *PairCoreSolverBase) AMultXIsOff() (off bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	return m.aMultXIsOffValue()
}
func (m *PairCoreSolverBase) aMultXIsOffValue() (off bool, err error) {
	if sourceDebugEnabled && len(*m.X) != int(m.A.ColumnCount()) {
		return false, ErrAssertion
	}
	for i := uint32(0); i < m.A.RowCount(); i++ {
		if uint64(i) >= uint64(len(*m.B)) {
			return false, ErrBounds
		}
		m.residualWork = NewIndexedPairVector(m.manager, 2)
		if err = m.A.DotPairRow(i, *m.X, &m.residualWork.Data[0]); err != nil {
			return false, err
		}
		if err = m.buildRowResidual(i); err != nil {
			return false, err
		}
		zero := NewRationalPair()
		equal, e := zero.Equal(m.manager, &m.residualWork.Data[1])
		if e != nil {
			return false, e
		}
		if !equal {
			if err = m.printRowResidual(i, os.Stdout); err != nil {
				return false, err
			}
		}
		if err = m.residualWork.Close(); err != nil {
			return false, err
		}
		m.residualWork = nil
		if !equal {
			return true, nil
		}
	}
	return false, nil
}
