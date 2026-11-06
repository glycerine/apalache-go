package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:185-247, concrete
// config_hwf midpoint node splitter.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

var ErrHardwareSplitNoInterior error = &sourceFailure{message: "subpaving: no representable hardware midpoint"}

// SplitMidpoint creates the two children before computing their shared
// midpoint, following the source order. On a later arithmetic refusal it
// returns the created children with the error so the caller can close them.
func (c *HardwareContext) SplitMidpoint(n *HardwareNode, variable uint32, leftOpen bool, delta uint32) (left, right *HardwareNode, err error) {
	if err = c.checkNode(n, variable); err != nil {
		return nil, nil, err
	}
	if n.conflictVar != math.MaxUint32 || len(c.queue) != 0 || c.queueHead != 0 || delta >= math.MaxInt32 {
		return nil, nil, arena.ErrInvalidSlot
	}
	left, err = c.NewChildNode(n)
	if err != nil {
		return nil, nil, err
	}
	right, err = c.NewChildNode(n)
	if err != nil {
		return left, nil, err
	}
	m := c.numbers
	lower, upper := n.lower[variable], n.upper[variable]
	var mid, step, two numeral.HardwareFloat
	switch {
	case lower == nil && upper == nil:
		err = m.SetInt32(&mid, 0)
	case lower == nil:
		if err = m.SetInt32(&step, int32(delta)); err != nil {
			return left, right, err
		}
		if err = m.Set(&mid, &upper.value); err != nil {
			return left, right, err
		}
		m.RoundToMinusInf()
		err = m.Sub(&mid, &step, &mid)
	case upper == nil:
		if err = m.SetInt32(&step, int32(delta)); err != nil {
			return left, right, err
		}
		if err = m.Set(&mid, &lower.value); err != nil {
			return left, right, err
		}
		m.RoundToPlusInf()
		err = m.Add(&mid, &step, &mid)
	default:
		if err = m.SetInt32(&two, 2); err != nil {
			return left, right, err
		}
		if err = m.Add(&lower.value, &upper.value, &mid); err != nil {
			return left, right, err
		}
		if err = m.Div(&mid, &two, &mid); err != nil {
			return left, right, err
		}
		if !m.Less(&lower.value, &mid) || !m.Less(&mid, &upper.value) {
			return left, right, ErrHardwareSplitNoInterior
		}
	}
	if err != nil {
		return left, right, err
	}
	if err = c.DecideBound(left, variable, &mid, false, leftOpen); err != nil {
		return left, right, err
	}
	if err = c.DecideBound(right, variable, &mid, true, !leftOpen); err != nil {
		return left, right, err
	}
	if err = c.traceSplitBounds(left, right); err != nil {
		return left, right, err
	}
	return left, right, nil
}
