package lp

// Source: src/util/lp/lp_core_solver_base.hpp:541-601,741-755.

import "fmt"

func (m *FloatCoreSolverBase) RestoreXAndRefactor(entering, leaving int32, t float64) error {
	if err := m.RestoreBasisChange(uint32(entering), uint32(leaving)); err != nil {
		return err
	}
	if err := m.RestoreX(uint32(entering), t); err != nil {
		return err
	}
	if err := m.InitLU(); err != nil {
		return err
	}
	if m.Factorization.GetStatus() == LUDegenerated {
		if m.Settings.DebugOut != nil {
			if _, err := fmt.Fprintln(m.Settings.DebugOut, "cannot refactor"); err != nil {
				return err
			}
		}
		m.Status = StatusFloatingPointError
		return nil
	}
	bad, err := m.AMultXIsOff()
	if err != nil {
		return err
	}
	if bad {
		if m.Settings.DebugOut != nil {
			if _, err := fmt.Fprintln(m.Settings.DebugOut, "cannot restore solution"); err != nil {
				return err
			}
		}
		m.Status = StatusFloatingPointError
	}
	return nil
}

// UpdateBasisAndX preserves the source's separate X recovery, Suhl replacement,
// full refactor and rollback paths. A false result is a source numerical outcome;
// errors qualify unsupported Go accesses and failures from actual child owners.
func (m *FloatCoreSolverBase) UpdateBasisAndX(entering, leaving int32, t float64) (bool, error) {
	en, lv := uint32(entering), uint32(leaving)
	if t != 0 {
		if err := m.UpdateX(en, t); err != nil {
			return false, err
		}
		bad, err := m.AMultXIsOffOnIndex(m.Ed.Index)
		if err != nil {
			return false, err
		}
		if bad {
			ok, err := m.FindXBySolving()
			if err != nil {
				return false, err
			}
			if !ok {
				if err := m.InitLU(); err != nil {
					return false, err
				}
				ok, err = m.FindXBySolving()
				if err != nil {
					return false, err
				}
				if !ok {
					if err := m.RestoreX(en, t); err != nil {
						return false, err
					}
					bad, err := m.AMultXIsOff()
					if err != nil {
						return false, err
					}
					if bad {
						m.Status = StatusFloatingPointError
						m.IterationsNoCostGrowth++
						return false, nil
					}
					if err := m.InitLU(); err != nil {
						return false, err
					}
					m.IterationsNoCostGrowth++
					if m.Factorization.GetStatus() != LUOK {
						m.Status = StatusFloatingPointError
					}
					return false, nil
				}
			}
		}
	}
	if m.Factorization == nil {
		return false, ErrBounds
	}
	if !m.Factorization.NeedToRefactor() {
		if uint64(en) >= uint64(len(m.PivotRow.Data)) || uint64(lv) >= uint64(len(*m.Heading)) {
			return false, ErrBounds
		}
		pivot := m.PivotRow.Data[en]
		if err := m.Factorization.ReplaceColumn(pivot, m.W, uint32((*m.Heading)[lv])); err != nil {
			return false, err
		}
		if m.Factorization.GetStatus() == LUOK {
			if err := m.ChangeBasis(en, lv); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	if err := m.ChangeBasis(en, lv); err != nil {
		return false, err
	}
	if err := m.InitLU(); err != nil {
		return false, err
	}
	if m.Factorization.GetStatus() != LUOK {
		if m.LookForFeasibleSolutionOnly {
			m.Status = StatusUnstable
			m.Factorization.Close()
			m.Factorization = nil
			return false, nil
		}
		if err := m.RestoreXAndRefactor(entering, leaving, t); err != nil {
			return false, err
		}
		if m.Status == StatusFloatingPointError {
			return false, nil
		}
		if sourceDebugEnabled {
			bad, err := m.AMultXIsOff()
			if err != nil {
				return false, err
			}
			if bad {
				return false, ErrAssertion
			}
		}
		m.IterationsNoCostGrowth++
		m.Status = StatusUnstable
		return false, nil
	}
	return true, nil
}
