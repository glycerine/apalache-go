package lp

// Source: src/util/lp/lp_primal_simplex.hpp:271-358;
// lp_primal_simplex.h:86-88. Retained floating named-solution validators.

import (
	"fmt"
	"io"
	"math"
	"sort"

	"github.com/glycerine/g3/pkg/util/z3exception"
)

func (m *FloatPrimalSimplex) BoundsHold(solution map[string]float64) (bool, error) {
	keys := make([]uint32, 0, len(m.Columns))
	for j := range m.Columns {
		keys = append(keys, j)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, j := range keys {
		c := m.Columns[j]
		if c == nil {
			return false, ErrBounds
		}
		v, ok := solution[c.Name]
		if !ok {
			return false, z3exception.NewDefaultException(fmt.Sprintf("cannot find column %d in solution", j))
		}
		if !c.BoundsHold(v) {
			c.BoundsHold(v)
			return false, nil
		}
	}
	return true, nil
}

// GetRowValue reads the original external coefficient dictionary, including
// retained zero entries, in its native insertion order. Writer errors return
// the sum accumulated before the refused source insertion.
func (m *FloatPrimalSimplex) GetRowValue(i uint32, solution map[string]float64, out io.Writer) (float64, error) {
	row, ok := m.AValues.Get2(i)
	if !ok {
		return 0, z3exception.NewDefaultException(fmt.Sprintf("cannot find row %d", i))
	}
	if row == nil {
		return 0, ErrBounds
	}
	ret := 0.0
	for j, a := range row.All() {
		c, ok := m.Columns[j]
		if !ok {
			if err := sourceAssertionFailure(); err != nil {
				return ret, err
			}
			return ret, ErrBounds
		}
		if c == nil {
			return ret, ErrBounds
		}
		value, ok := solution[c.Name]
		if !ok {
			if err := sourceAssertionFailure(); err != nil {
				return ret, err
			}
			return ret, ErrBounds
		}
		if out != nil {
			if err := writeConstraintText(out, fmt.Sprintf("%s(%s=%s) ", floatLPNumberText(a), c.Name, floatLPNumberText(value))); err != nil {
				return ret, err
			}
		}
		ret += a * value
	}
	if out != nil {
		if err := writeDriverLine(out, " = "+floatLPNumberText(ret)); err != nil {
			return ret, err
		}
	}
	return ret, nil
}

func (m *FloatPrimalSimplex) RowConstraintHolds(i uint32, solution map[string]float64, out io.Writer) (bool, error) {
	value, err := m.GetRowValue(i, solution, out)
	if err != nil {
		return false, err
	}
	c, ok := m.Constraints[i]
	if !ok {
		// Source operator[] inserts an indeterminate default constraint.
		// Publish the key, then guard its undefined double/enum reads.
		m.Constraints[i] = FloatConstraint{}
		return false, ErrBounds
	}
	var comparison string
	bad := false
	switch c.Relation {
	case RelationEqual:
		bad = math.Abs(value-c.RightSide) > 0.00001
		comparison = "="
	case RelationGreaterOrEqual:
		bad = value-c.RightSide < -0.00001
		comparison = ">="
	case RelationLessOrEqual:
		bad = value-c.RightSide > 0.00001
		comparison = "<="
	default:
		return false, sourceAssertionFailure()
	}
	if bad {
		if out != nil {
			if err := writeDriverLine(out, "should be "+comparison+" "+floatLPNumberText(c.RightSide)); err != nil {
				return false, err
			}
		}
		return false, nil
	}
	return true, nil
}

func (m *FloatPrimalSimplex) RowConstraintsHold(solution map[string]float64) (bool, error) {
	for i := range m.AValues.All() {
		ok, err := m.RowConstraintHolds(i, solution, nil)
		if err != nil {
			return false, err
		}
		if !ok {
			_, err = m.RowConstraintHolds(i, solution, nil)
			return false, err
		}
	}
	return true, nil
}

func (m *FloatPrimalSimplex) SolutionIsFeasible(solution map[string]float64) (bool, error) {
	ok, err := m.BoundsHold(solution)
	if err != nil || !ok {
		return false, err
	}
	return m.RowConstraintsHold(solution)
}
