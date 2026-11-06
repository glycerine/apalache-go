package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:657-664,690-721,1500-1663,
// concrete config_hwf monomial construction and propagation.

import (
	"errors"
	"math"
	"sort"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type HardwarePower struct {
	Variable uint32
	Degree   uint32
}

type HardwareMonomial struct {
	powers  []HardwarePower
	visited uint64
}

// DefineMonomial merges equal variables after sorting. The source marks the
// result integer if any factor variable is integer.
func (c *HardwareContext) DefineMonomial(powers []HardwarePower) (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	if len(powers) == 0 || c.root != nil || len(c.isInt) >= math.MaxUint32 {
		return 0, arena.ErrInvalidSlot
	}
	merged := append([]HardwarePower(nil), powers...)
	for _, power := range merged {
		if power.Variable >= uint32(len(c.isInt)) {
			return 0, arena.ErrInvalidSlot
		}
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].Variable < merged[j].Variable })
	write := 0
	for read := 1; read < len(merged); read++ {
		if merged[write].Variable == merged[read].Variable {
			merged[write].Degree += merged[read].Degree
		} else {
			write++
			merged[write] = merged[read]
		}
	}
	merged = merged[:write+1]
	integer := false
	for _, power := range merged {
		integer = integer || c.isInt[power.Variable]
	}
	x := uint32(len(c.isInt))
	c.isInt = append(c.isInt, integer)
	c.watchers = append(c.watchers, nil)
	c.watchOrder = append(c.watchOrder, nil)
	if c.monomials == nil {
		c.monomials = make(map[uint32]*HardwareMonomial)
	}
	c.monomials[x] = &HardwareMonomial{powers: merged}
	for _, power := range merged {
		c.watchers[power.Variable] = append(c.watchers[power.Variable], x)
		c.watchOrder[power.Variable] = append(c.watchOrder[power.Variable], hardwareWatch{definition: x})
	}
	return x, nil
}

func (n *HardwareNode) IsUnbounded(variable uint32) (bool, error) {
	lo, err := n.bound(variable, true)
	if err != nil {
		return false, err
	}
	hi, err := n.bound(variable, false)
	return lo == nil && hi == nil, err
}

func (n *HardwareNode) IsZero(variable uint32) (bool, error) {
	lo, err := n.bound(variable, true)
	if err != nil {
		return false, err
	}
	hi, err := n.bound(variable, false)
	if err != nil || lo == nil || hi == nil {
		return false, err
	}
	return n.numbers.IsZero(&lo.value) && n.numbers.IsZero(&hi.value) && !lo.open && !hi.open, nil
}

func (n *HardwareNode) IsUpperZero(variable uint32) (bool, error) {
	hi, err := n.bound(variable, false)
	if err != nil || hi == nil {
		return false, err
	}
	return n.numbers.IsZero(&hi.value) && !hi.open, nil
}

func (c *HardwareContext) PropagateMonomial(variable uint32, n *HardwareNode) (err error) {
	if err := c.checkNode(n, variable); err != nil {
		return err
	}
	if n.conflictVar != math.MaxUint32 {
		return arena.ErrInvalidSlot
	}
	m := c.monomials[variable]
	if m == nil {
		return arena.ErrInvalidSlot
	}
	if err = c.traceMonomialStart(variable); err != nil {
		return err
	}
	m.visited = c.timestamp
	foundZero, foundUnbounded := false, false
	for _, power := range m.powers {
		zero, err := n.IsZero(power.Variable)
		if err != nil {
			return err
		}
		foundZero = foundZero || zero
		if power.Degree%2 == 0 {
			upperZero, err := n.IsUpperZero(power.Variable)
			if err != nil {
				return err
			}
			foundZero = foundZero || upperZero
			continue
		}
		unbounded, err := n.IsUnbounded(power.Variable)
		if err != nil {
			return err
		}
		foundUnbounded = foundUnbounded || unbounded
	}
	if err = c.traceMonomialFlags(foundZero, foundUnbounded); err != nil {
		return err
	}
	if foundZero {
		zero, err := n.IsZero(variable)
		if err != nil || zero {
			return err
		}
		var value numeral.HardwareFloat
		if err := c.numbers.SetInt32(&value, 0); err != nil {
			return err
		}
		if err := c.propagateBoundReason(n, variable, &value, true, false, hardwareReasonVariableDefinition, variable); err != nil || n.conflictVar != math.MaxUint32 {
			return err
		}
		return c.propagateBoundReason(n, variable, &value, false, false, hardwareReasonVariableDefinition, variable)
	}
	xUnbounded, err := n.IsUnbounded(variable)
	if err != nil {
		return err
	}
	if !foundUnbounded {
		if err := c.propagateMonomialUpward(m, variable, n); err != nil || n.conflictVar != math.MaxUint32 {
			return err
		}
	}
	if xUnbounded {
		return nil
	}
	aux, err := NewHardwareInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, aux.Close()) }()
	badPosition := -1
	for i, power := range m.powers {
		if err := aux.SetConstant(n, power.Variable); err != nil {
			return err
		}
		containsZero, err := aux.ContainsZero()
		if err != nil {
			return err
		}
		if containsZero {
			if badPosition >= 0 {
				return nil
			}
			badPosition = i
		}
	}
	if badPosition >= 0 {
		return c.propagateMonomialDownward(m, variable, badPosition, n)
	}
	for i := range m.powers {
		if n.conflictVar != math.MaxUint32 {
			return nil
		}
		if err := c.propagateMonomialDownward(m, variable, i, n); err != nil {
			return err
		}
	}
	return nil
}

func (c *HardwareContext) propagateMonomialUpward(m *HardwareMonomial, variable uint32, n *HardwareNode) (err error) {
	r, err := NewHardwareInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, r.Close()) }()
	y, err := NewHardwareInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, y.Close()) }()
	yk, err := NewHardwareInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, yk.Close()) }()
	for i, power := range m.powers {
		if err = y.SetConstant(n, power.Variable); err != nil {
			return err
		}
		if err = yk.Power(y, power.Degree); err != nil {
			return err
		}
		if i == 0 {
			err = r.SetFrom(yk)
		} else {
			err = r.Mul(r, yk)
		}
		if err != nil {
			return err
		}
	}
	return c.propagateHardwareIntervalBounds(n, variable, variable, r)
}

func (c *HardwareContext) propagateMonomialDownward(m *HardwareMonomial, variable uint32, j int, n *HardwareNode) (err error) {
	if err = c.traceMonomialDownward(variable, j); err != nil {
		return err
	}
	r, err := NewHardwareInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, r.Close()) }()
	y, err := NewHardwareInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, y.Close()) }()
	if len(m.powers) > 1 {
		d, e := NewHardwareInterval(c.numbers)
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, d.Close()) }()
		first := true
		for i, power := range m.powers {
			if i == j {
				continue
			}
			if err = y.SetConstant(n, power.Variable); err != nil {
				return err
			}
			if err = r.Power(y, power.Degree); err != nil {
				return err
			}
			if first {
				err = d.SetFrom(r)
				first = false
			} else {
				err = r.Mul(d, r)
			}
			if err != nil {
				return err
			}
		}
		if err = y.SetConstant(n, variable); err != nil {
			return err
		}
		if err = r.Div(y, d); err != nil {
			return err
		}
	} else {
		if err = y.SetConstant(n, variable); err != nil {
			return err
		}
		if err = r.SetFrom(y); err != nil {
			return err
		}
	}
	degree := m.powers[j].Degree
	if degree > 1 {
		if degree%2 == 0 {
			negative, e := r.LowerIsNeg()
			if e != nil {
				return e
			}
			if negative {
				return nil
			}
		}
		if err = c.XnEqY(r, r, degree); err != nil {
			return err
		}
	}
	return c.propagateHardwareIntervalBounds(n, m.powers[j].Variable, variable, r)
}

func (c *HardwareContext) propagateHardwareIntervalBounds(n *HardwareNode, target, source uint32, r *HardwareInterval) error {
	lo, hi, err := r.endpoints()
	if err != nil {
		return err
	}
	if !r.lowerInf {
		if _, err := c.propagateCandidateReason(n, target, lo, true, r.lowerOpen, hardwareReasonVariableDefinition, source); err != nil {
			return err
		}
		if n.conflictVar != math.MaxUint32 {
			return nil
		}
	}
	if !r.upperInf {
		_, err = c.propagateCandidateReason(n, target, hi, false, r.upperOpen, hardwareReasonVariableDefinition, source)
	}
	return err
}
