package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:185-247, concrete config_mpf
// midpoint node splitter.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

var ErrArbitrarySplitNoInterior error = &sourceFailure{message: "subpaving: no representable arbitrary midpoint"}

// SplitMidpoint creates both children before calculating the source midpoint.
// Arithmetic or no-interior refusal leaves the children owned by the context.
func (c *ArbitraryContext) SplitMidpoint(n *ArbitraryNode, variable uint32, leftOpen bool, delta uint32) (left, right *ArbitraryNode, err error) {
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
	var mid, step, two numeral.ArbitraryFloat
	defer func() { err = errors.Join(err, m.Del(&mid), m.Del(&step), m.Del(&two)) }()
	lower, upper := n.lower[variable], n.upper[variable]
	switch {
	case lower == nil && upper == nil:
		err = m.SetInt32(&mid, 0)
	case lower == nil:
		if err = m.SetInt32(&step, int32(delta)); err != nil {
			return left, right, err
		}
		if err = m.Copy(&mid, &upper.value); err != nil {
			return left, right, err
		}
		m.RoundToMinusInf()
		err = m.Sub(&mid, &step, &mid)
	case upper == nil:
		if err = m.SetInt32(&step, int32(delta)); err != nil {
			return left, right, err
		}
		if err = m.Copy(&mid, &lower.value); err != nil {
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
		leftInterior, e := m.Less(&lower.value, &mid)
		if e != nil {
			return left, right, e
		}
		rightInterior, e := m.Less(&mid, &upper.value)
		if e != nil {
			return left, right, e
		}
		if !leftInterior || !rightInterior {
			return left, right, ErrArbitrarySplitNoInterior
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
