package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:185-247, concrete config_mpq
// midpoint splitting and source decision-bound normalization.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// DecideBound installs an axiom decision without enqueueing it. Search later
// enqueues only the most recent bounds since the parent trail.
func (c *RationalContext) DecideBound(n *RationalNode, variable uint32, value *numeral.Rational, lower, open bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if n == nil || n.context != c || !n.registered || n == c.root || n.parent == nil || n.conflictVar != math.MaxUint32 || len(c.queue) != 0 || c.queueHead != 0 {
		return arena.ErrInvalidSlot
	}
	return c.publishBoundReason(n, variable, value, lower, open, true, nil, math.MaxUint32)
}

// SplitMidpoint creates both children before arithmetic, as the source does.
// A later refusal leaves them owned by the context for caller cleanup.
func (c *RationalContext) SplitMidpoint(n *RationalNode, variable uint32, leftOpen bool, delta uint32) (left, right *RationalNode, err error) {
	if err = c.open(); err != nil {
		return nil, nil, err
	}
	if n == nil || n.context != c || !n.registered || variable >= uint32(len(c.variables)) || n.conflictVar != math.MaxUint32 || len(c.queue) != 0 || c.queueHead != 0 || delta >= math.MaxInt32 {
		return nil, nil, arena.ErrInvalidSlot
	}
	if err = n.open(); err != nil {
		return nil, nil, err
	}
	left, err = c.NewChild(n)
	if err != nil {
		return nil, nil, err
	}
	right, err = c.NewChild(n)
	if err != nil {
		return left, nil, err
	}
	mid, step, two := numeral.NewRational(), numeral.NewRational(), numeral.NewRational()
	defer func() { err = errors.Join(err, c.numbers.Del(&two), c.numbers.Del(&step), c.numbers.Del(&mid)) }()
	m := c.numbers
	lo, hi := n.lower[variable], n.upper[variable]
	switch {
	case lo == nil && hi == nil:
		err = m.SetInt32(&mid, 0)
	case lo == nil:
		if err = m.SetInt32(&step, int32(delta)); err != nil {
			return left, right, err
		}
		if err = m.Copy(&mid, &hi.value); err != nil {
			return left, right, err
		}
		err = m.Sub(&mid, &step, &mid)
	case hi == nil:
		if err = m.SetInt32(&step, int32(delta)); err != nil {
			return left, right, err
		}
		if err = m.Copy(&mid, &lo.value); err != nil {
			return left, right, err
		}
		err = m.Add(&mid, &step, &mid)
	default:
		if err = m.SetInt32(&two, 2); err != nil {
			return left, right, err
		}
		if err = m.Add(&lo.value, &hi.value, &mid); err != nil {
			return left, right, err
		}
		if err = m.Div(&mid, &two, &mid); err != nil {
			return left, right, err
		}
		leftComparison, e := m.Compare(&lo.value, &mid)
		if e != nil {
			return left, right, e
		}
		rightComparison, e := m.Compare(&mid, &hi.value)
		if e != nil {
			return left, right, e
		}
		if leftComparison >= 0 || rightComparison >= 0 {
			return left, right, ErrSubpavingFailure
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
