package lp

// Source: src/util/lp/lp_core_solver_base.hpp:543-600,741-755,916-918;
// lp_core_solver_base.h:509-514. Retained scalar mpq/mpq specialization.

import (
	"fmt"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (m *RationalCoreSolverBase) InitLU() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return InitRationalFactorization(&m.Factorization, m.A, *m.Basis, m.Settings)
}

func (m *RationalCoreSolverBase) RestoreXAndRefactor(entering, leaving int32, t *numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if err = m.RestoreBasisChange(uint32(entering), uint32(leaving)); err != nil {
		return err
	}
	if err = m.RestoreX(uint32(entering), t); err != nil {
		return err
	}
	if err = m.InitLU(); err != nil {
		return err
	}
	if m.Factorization.GetStatus() == LUDegenerated {
		if m.Settings == nil {
			return ErrBounds
		}
		if m.Settings.DebugOut != nil {
			if _, err = fmt.Fprintln(m.Settings.DebugOut, "cannot refactor"); err != nil {
				return err
			}
		}
		m.Status = StatusFloatingPointError
		return nil
	}
	off, e := m.AMultXIsOff()
	if e != nil {
		return e
	}
	if off {
		if m.Settings == nil {
			return ErrBounds
		}
		if m.Settings.DebugOut != nil {
			if _, err = fmt.Fprintln(m.Settings.DebugOut, "cannot restore solution"); err != nil {
				return err
			}
		}
		m.Status = StatusFloatingPointError
	}
	return nil
}

// UpdateBasisAndX keeps precise value updates, LU column replacement, full
// refactor and rollback in source order. False without error is a source
// numerical outcome. Exact mode always rolls back a failed new basis, including
// feasibility-only mode; it never uses the floating residual repair branch.
func (m *RationalCoreSolverBase) UpdateBasisAndX(entering, leaving int32, t *numeral.Rational) (ok bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	if t == nil {
		return false, ErrBounds
	}
	zero, e := m.manager.IsZero(t)
	if e != nil {
		return false, e
	}
	en, lv := uint32(entering), uint32(leaving)
	if !zero {
		if err = m.UpdateX(en, t); err != nil {
			return false, err
		}
	}
	if m.Factorization == nil {
		return false, ErrBounds
	}
	if !m.Factorization.NeedToRefactor() {
		if uint64(en) >= uint64(len(m.PivotRow.Data)) || uint64(lv) >= uint64(len(*m.Heading)) {
			return false, ErrBounds
		}
		pivot := &m.PivotRow.Data[en]
		if err = m.Factorization.ReplaceColumn(pivot, m.W, uint32((*m.Heading)[lv])); err != nil {
			return false, err
		}
		if m.Factorization.GetStatus() == LUOK {
			if err = m.ChangeBasis(en, lv); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	if err = m.ChangeBasis(en, lv); err != nil {
		return false, err
	}
	if err = m.InitLU(); err != nil {
		return false, err
	}
	if m.Factorization.GetStatus() != LUOK {
		if err = m.RestoreXAndRefactor(entering, leaving, t); err != nil {
			return false, err
		}
		if m.Status == StatusFloatingPointError {
			return false, nil
		}
		if sourceDebugEnabled {
			off, e := m.AMultXIsOff()
			if e != nil {
				return false, e
			}
			if off {
				return false, ErrAssertion
			}
		}
		m.IterationsNoCostGrowth++
		m.Status = StatusUnstable
		return false, nil
	}
	return true, nil
}
