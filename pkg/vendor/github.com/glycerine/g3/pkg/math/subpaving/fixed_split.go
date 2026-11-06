package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:185-247, concrete
// config_mpfx midpoint node splitter.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

var ErrFixedSplitNoInterior error = &sourceFailure{message: "subpaving: no representable strict midpoint"}

// SplitMidpoint creates both children before midpoint arithmetic, as in the
// source. The caller owns returned children even when later arithmetic fails.
// A selected child runs AddRecentBounds followed by PropagateQueued.
func (c *FixedContext) SplitMidpoint(n *FixedNode, variable uint32, leftOpen bool, delta uint32) (left, right *FixedNode, err error) {
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
	mid, err := m.New()
	if err != nil {
		return left, right, err
	}
	defer func() { err = errors.Join(err, mid.Close()) }()
	lower, upper := n.lower[variable], n.upper[variable]
	switch {
	case lower == nil && upper == nil:
		err = m.SetInt64(mid, 0)
	case lower == nil || upper == nil:
		step, e := m.New()
		if e != nil {
			return left, right, e
		}
		defer func() { err = errors.Join(err, step.Close()) }()
		if err = m.SetInt64(step, int64(delta)); err != nil {
			return left, right, err
		}
		if lower == nil {
			if err = m.SetRounding(false); err != nil {
				return left, right, err
			}
			err = m.Sub(upper.value, step, mid)
		} else {
			if err = m.SetRounding(true); err != nil {
				return left, right, err
			}
			err = m.Add(lower.value, step, mid)
		}
	default:
		two, e := m.New()
		if e != nil {
			return left, right, e
		}
		defer func() { err = errors.Join(err, two.Close()) }()
		if err = m.SetInt64(two, 2); err != nil {
			return left, right, err
		}
		if err = m.Add(lower.value, upper.value, mid); err != nil {
			return left, right, err
		}
		if err = m.Div(mid, two, mid); err != nil {
			return left, right, err
		}
		loCmp, e := m.Compare(lower.value, mid)
		if e != nil {
			return left, right, e
		}
		hiCmp, e := m.Compare(mid, upper.value)
		if e != nil {
			return left, right, e
		}
		if loCmp >= 0 || hiCmp >= 0 {
			return left, right, ErrFixedSplitNoInterior
		}
	}
	if err != nil {
		return left, right, err
	}
	if err = c.DecideBound(left, variable, mid, false, leftOpen); err != nil {
		return left, right, err
	}
	if err = c.DecideBound(right, variable, mid, true, !leftOpen); err != nil {
		return left, right, err
	}
	if err = c.traceSplitBounds(left, right); err != nil {
		return left, right, err
	}
	return left, right, nil
}
