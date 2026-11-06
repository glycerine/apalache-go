package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1500-1663, concrete config_mpf
// monomial propagation over live root-bound interval views.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (c *ArbitraryContext) PropagateMonomial(variable uint32, n *ArbitraryNode) (err error) {
	if err = c.checkNode(n, variable); err != nil {
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
	c.numVisited++
	m.visited = c.timestamp
	foundZero, foundUnbounded := false, false
	for _, power := range m.powers {
		zero, e := n.IsZero(power.Variable)
		if e != nil {
			return e
		}
		foundZero = foundZero || zero
		if power.Degree%2 == 0 {
			upperZero, e := n.IsUpperZero(power.Variable)
			if e != nil {
				return e
			}
			foundZero = foundZero || upperZero
			continue
		}
		unbounded := n.lower[power.Variable] == nil && n.upper[power.Variable] == nil
		foundUnbounded = foundUnbounded || unbounded
	}
	if err = c.traceMonomialFlags(foundZero, foundUnbounded); err != nil {
		return err
	}
	if foundZero {
		alreadyZero, e := n.IsZero(variable)
		if e != nil || alreadyZero {
			return e
		}
		var zero numeral.ArbitraryFloat
		defer func() { err = errors.Join(err, c.numbers.Del(&zero)) }()
		if err = c.numbers.SetInt32(&zero, 0); err != nil {
			return err
		}
		if err = c.publishBoundWithReason(n, variable, &zero, true, false, false, nil, variable, true, true); err != nil || n.conflictVar != math.MaxUint32 {
			return err
		}
		return c.publishBoundWithReason(n, variable, &zero, false, false, false, nil, variable, true, true)
	}
	xUnbounded := n.lower[variable] == nil && n.upper[variable] == nil
	if !foundUnbounded {
		if err = c.propagateMonomialUpward(m, variable, n); err != nil || n.conflictVar != math.MaxUint32 {
			return err
		}
	}
	if xUnbounded {
		return nil
	}
	aux, err := NewArbitraryInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, aux.Close()) }()
	badPosition := -1
	for i, power := range m.powers {
		if err = aux.SetConstant(n, power.Variable); err != nil {
			return err
		}
		containsZero, e := aux.ContainsZero()
		if e != nil {
			return e
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
		if err = c.propagateMonomialDownward(m, variable, i, n); err != nil {
			return err
		}
	}
	return nil
}

func (c *ArbitraryContext) propagateMonomialUpward(m *ArbitraryMonomial, variable uint32, n *ArbitraryNode) (err error) {
	r, err := NewArbitraryInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, r.Close()) }()
	y, err := NewArbitraryInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, y.Close()) }()
	yk, err := NewArbitraryInterval(c.numbers)
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
	lo, hi, err := r.endpoints()
	if err != nil {
		return err
	}
	return c.propagateArbitraryIntervalBounds(n, variable, variable, r, lo, hi)
}

func (c *ArbitraryContext) propagateMonomialDownward(m *ArbitraryMonomial, variable uint32, j int, n *ArbitraryNode) (err error) {
	if err = c.traceMonomialDownward(variable, j); err != nil {
		return err
	}
	r, err := NewArbitraryInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, r.Close()) }()
	y, err := NewArbitraryInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, y.Close()) }()
	if len(m.powers) > 1 {
		d, e := NewArbitraryInterval(c.numbers)
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
	lo, hi, err := r.endpoints()
	if err != nil {
		return err
	}
	return c.propagateArbitraryIntervalBounds(n, m.powers[j].Variable, variable, r, lo, hi)
}
