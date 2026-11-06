package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:185-247; concrete config_mpff
// midpoint node splitter.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

var ErrFastSplitNoInterior error = &sourceFailure{message: "subpaving: no representable fast-float midpoint"}

// SplitMidpoint creates both children before computing the source midpoint.
// A later numeric refusal leaves the created children for caller cleanup.
func (c *FastContext) SplitMidpoint(n *FastNode, variable uint32, leftOpen bool, delta uint32) (left, right *FastNode, err error) {
	if err = c.checkNode(n, variable); err != nil {
		return nil, nil, err
	}
	if n.inconsistent || len(c.queue) != 0 || c.queueHead != 0 || delta >= math.MaxInt32 {
		return nil, nil, arena.ErrInvalidSlot
	}
	left, err = c.NewChild(n)
	if err != nil {
		return nil, nil, err
	}
	right, err = c.NewChild(n)
	if err != nil {
		return left, nil, err
	}
	m := c.numbers
	var mid, step, two numeral.FastFloat
	defer func() { err = errors.Join(err, m.Del(&mid), m.Del(&step), m.Del(&two)) }()
	lower, upper := n.lower[variable], n.upper[variable]
	switch {
	case lower == nil && upper == nil:
		err = m.SetInt32(&mid, 0)
	case lower == nil:
		if err = m.SetInt32(&step, int32(delta)); err != nil {
			return left, right, err
		}
		if err = m.Copy(&mid, upper.value); err != nil {
			return left, right, err
		}
		if err = m.RoundToMinusInf(); err != nil {
			return left, right, err
		}
		err = m.Sub(&mid, &step, &mid)
	case upper == nil:
		if err = m.SetInt32(&step, int32(delta)); err != nil {
			return left, right, err
		}
		if err = m.Copy(&mid, lower.value); err != nil {
			return left, right, err
		}
		if err = m.RoundToPlusInf(); err != nil {
			return left, right, err
		}
		err = m.Add(&mid, &step, &mid)
	default:
		if err = m.SetInt32(&two, 2); err != nil {
			return left, right, err
		}
		if err = m.Add(lower.value, upper.value, &mid); err != nil {
			return left, right, err
		}
		if err = m.Div(&mid, &two, &mid); err != nil {
			return left, right, err
		}
		leftInterior, e := m.Less(lower.value, &mid)
		if e != nil {
			return left, right, e
		}
		rightInterior, e := m.Less(&mid, upper.value)
		if e != nil {
			return left, right, e
		}
		if !leftInterior || !rightInterior {
			return left, right, ErrFastSplitNoInterior
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
