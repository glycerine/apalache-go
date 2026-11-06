package lp

// Source: src/util/lp/lp_solver.hpp:168-412, retained floating presolve.

import (
	"fmt"

	"github.com/glycerine/insdict"
)

func (m *FloatLPSolver) PinVarsOnRowWithSign(row *insdict.Dict[uint32, float64], sign float64) error {
	for j, a := range row.All() {
		c, ok := m.Columns[j]
		if !ok {
			m.Columns[j] = nil
		}
		if c == nil {
			return ErrBounds
		}
		var v float64
		var err error
		if a*sign > 0 {
			v, err = c.GetUpperBound()
		} else {
			v, err = c.GetLowBound()
		}
		if err != nil {
			return err
		}
		c.SetFixedValue(v)
	}
	return nil
}

// The returned value includes the successfully accumulated prefix when a
// missing bound makes the row extremum unknown, just like the source out-ref.
func (m *FloatLPSolver) rowBound(row *insdict.Dict[uint32, float64], maximum bool) (float64, bool, error) {
	var sum float64
	for j, a := range row.All() {
		c, ok := m.Columns[j]
		if !ok {
			m.Columns[j] = nil
		}
		if c == nil {
			return sum, false, ErrBounds
		}
		low := a > 0
		if maximum {
			low = a < 0
		}
		if low {
			if !c.LowSet {
				return sum, false, nil
			}
			sum += c.Low * a
		} else {
			if !c.UpperSet {
				return sum, false, nil
			}
			sum += c.Upper * a
		}
	}
	return sum, true, nil
}
func (m *FloatLPSolver) MinimalRowValue(row *insdict.Dict[uint32, float64]) (float64, bool, error) {
	return m.rowBound(row, false)
}
func (m *FloatLPSolver) MaximalRowValue(row *insdict.Dict[uint32, float64]) (float64, bool, error) {
	return m.rowBound(row, true)
}

func floatSolverRowIsZero(row *insdict.Dict[uint32, float64]) bool {
	for _, a := range row.All() {
		if a != 0 {
			return false
		}
	}
	return true
}

func (m *FloatLPSolver) rowBoundObsolete(row *insdict.Dict[uint32, float64], rhs float64, upper bool) (bool, error) {
	bound, known, err := m.rowBound(row, upper)
	if err != nil {
		return false, err
	}
	if !known {
		return false, nil
	}
	diff := bound - rhs
	if upper {
		diff = rhs - bound
	}
	// This is a directional comparison, NOT an absolute epsilon test.
	if !(diff < m.Settings.RefactorTolerance) {
		m.Status = StatusInfeasible
		return true, nil
	}
	if -diff < m.Settings.RefactorTolerance {
		sign := -1.
		if upper {
			sign = 1
		}
		if err := m.PinVarsOnRowWithSign(row, sign); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (m *FloatLPSolver) RowIsObsolete(row *insdict.Dict[uint32, float64], index uint32) (bool, error) {
	c, ok := m.Constraints[index]
	if !ok {
		// Source operator[] constructs an indeterminate floating constraint.
		// Retain insertion, then guard its first undefined relation/value read.
		m.Constraints[index] = FloatConstraint{}
		return false, ErrBounds
	}
	switch c.Relation {
	case RelationEqual, RelationGreaterOrEqual, RelationLessOrEqual:
	default:
		return false, sourceAssertionFailure()
	}
	if floatSolverRowIsZero(row) {
		bad := c.RightSide != 0
		if c.Relation == RelationGreaterOrEqual {
			bad = c.RightSide > 0
		}
		if c.Relation == RelationLessOrEqual {
			bad = c.RightSide < 0
		}
		if bad {
			m.Status = StatusInfeasible
		}
		return true, nil
	}
	if c.Relation != RelationGreaterOrEqual {
		obsolete, err := m.rowBoundObsolete(row, c.RightSide, false)
		if err != nil || obsolete {
			return obsolete, err
		}
	}
	if c.Relation != RelationLessOrEqual {
		return m.rowBoundObsolete(row, c.RightSide, true)
	}
	return false, nil
}

func (m *FloatLPSolver) RemoveFixedOrZeroColumnsFromRow(index uint32, row *insdict.Dict[uint32, float64]) error {
	c, initialized := m.Constraints[index]
	if !initialized {
		m.Constraints[index] = c
	}
	var removed []uint32
	for j, a := range row.All() {
		ci, exists := m.Columns[j]
		if !exists {
			if err := sourceAssertionFailure(); err != nil {
				return err
			}
			m.Columns[j] = nil
		}
		if ci == nil {
			return ErrBounds
		}
		if ci.Fixed {
			removed = append(removed, j)
			if !initialized {
				return ErrBounds
			}
			c.RightSide -= a * ci.FixedValue
			// Publish every subtraction before inspecting the next coefficient.
			m.Constraints[index] = c
		} else if a == 0 {
			removed = append(removed, j)
		}
	}
	for _, j := range removed {
		row.Del(j)
	}
	return nil
}
func (m *FloatLPSolver) RemoveFixedOrZeroColumns() error {
	for i, row := range m.AValues.All() {
		if err := m.RemoveFixedOrZeroColumnsFromRow(i, row); err != nil {
			return err
		}
	}
	return nil
}

func (m *FloatLPSolver) TryToRemoveSomeRows() (uint32, error) {
	var removed []uint32
	for i, row := range m.AValues.All() {
		obsolete, err := m.RowIsObsolete(row, i)
		if err != nil {
			return 0, err
		}
		if obsolete {
			removed = append(removed, i)
		}
		if m.Status == StatusInfeasible {
			return 0, nil
		}
	}
	for _, i := range removed {
		m.AValues.Del(i)
	}
	if err := m.RemoveFixedOrZeroColumns(); err != nil {
		return 0, err
	}
	return uint32(len(removed)), nil
}

func (m *FloatLPSolver) Cleanup() error {
	passes := 0
	for {
		n, err := m.TryToRemoveSomeRows()
		if err != nil {
			return err
		}
		if n == 0 {
			break
		}
		passes++
	}
	// Source d=(try_to_remove_some_rows()>0) counts successful passes, not rows.
	if m.Settings.DebugOut == nil || passes == 0 {
		return nil
	}
	if passes == 1 {
		return writeDriverLine(m.Settings.DebugOut, "deleted one row")
	}
	return writeDriverLine(m.Settings.DebugOut, fmt.Sprintf("deleted %d rows", passes))
}
