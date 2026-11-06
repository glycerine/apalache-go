package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1500-1663, concrete config_mpq
// monomial propagation over live bound intervals.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (c *RationalContext) PropagateMonomial(variable uint32, n *RationalNode) (err error) {
	if err = c.open(); err != nil {
		return err
	}
	if n == nil || n.context != c || !n.registered || variable >= uint32(len(c.variables)) || n.conflictVar != math.MaxUint32 {
		return arena.ErrInvalidSlot
	}
	if err = n.open(); err != nil {
		return err
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
		} else if n.lower[power.Variable] == nil && n.upper[power.Variable] == nil {
			foundUnbounded = true
		}
	}
	if err = c.traceMonomialFlags(foundZero, foundUnbounded); err != nil {
		return err
	}
	if foundZero {
		alreadyZero, e := n.IsZero(variable)
		if e != nil || alreadyZero {
			return e
		}
		zero := numeral.NewRational()
		defer func() { err = errors.Join(err, c.numbers.Del(&zero)) }()
		if err = c.numbers.SetInt32(&zero, 0); err != nil {
			return err
		}
		if err = c.propagateMonomialZero(n, variable, &zero, true); err != nil || n.conflictVar != math.MaxUint32 {
			return err
		}
		return c.propagateMonomialZero(n, variable, &zero, false)
	}
	xUnbounded := n.lower[variable] == nil && n.upper[variable] == nil
	if !foundUnbounded {
		if err = c.propagateMonomialUpward(m, n); err != nil || n.conflictVar != math.MaxUint32 {
			return err
		}
	}
	if xUnbounded {
		return nil
	}
	aux, e := NewRationalInterval(c.numbers)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, aux.Close()) }()
	bad := -1
	for i, power := range m.powers {
		if err = aux.SetConstant(n, power.Variable); err != nil {
			return err
		}
		containsZero, e := aux.ContainsZero()
		if e != nil {
			return e
		}
		if containsZero {
			if bad >= 0 {
				return nil
			}
			bad = i
		}
	}
	if bad >= 0 {
		return c.propagateMonomialDownward(m, bad, n)
	}
	for i := range m.powers {
		if n.conflictVar != math.MaxUint32 {
			return nil
		}
		if err = c.propagateMonomialDownward(m, i, n); err != nil {
			return err
		}
	}
	return nil
}

func (c *RationalContext) propagateMonomialZero(n *RationalNode, variable uint32, zero *numeral.Rational, lower bool) error {
	if err := c.publishBoundReason(n, variable, zero, lower, false, false, nil, variable); err != nil {
		return err
	}
	c.queue = append(c.queue, n.trail)
	return nil
}

func (c *RationalContext) propagateMonomialInterval(n *RationalNode, target, definition uint32, r *RationalInterval) error {
	lo, hi, err := r.resolved()
	if err != nil {
		return err
	}
	if !r.lowerInf {
		if err = c.propagateRationalDefinitionCandidate(n, target, definition, lo, true, r.lowerOpen); err != nil || n.conflictVar != math.MaxUint32 {
			return err
		}
	}
	if !r.upperInf {
		return c.propagateRationalDefinitionCandidate(n, target, definition, hi, false, r.upperOpen)
	}
	return nil
}

func (c *RationalContext) propagateMonomialUpward(m *RationalMonomial, n *RationalNode) (err error) {
	r, err := NewRationalInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, r.Close()) }()
	y, err := NewRationalInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, y.Close()) }()
	yk, err := NewRationalInterval(c.numbers)
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
	return c.propagateMonomialInterval(n, m.variable, m.variable, r)
}

func (c *RationalContext) propagateMonomialDownward(m *RationalMonomial, j int, n *RationalNode) (err error) {
	if err = c.traceMonomialDownward(m.variable, j); err != nil {
		return err
	}
	r, err := NewRationalInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, r.Close()) }()
	y, err := NewRationalInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, y.Close()) }()
	if len(m.powers) > 1 {
		d, e := NewRationalInterval(c.numbers)
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, d.Close()) }()
		yk, e := NewRationalInterval(c.numbers)
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, yk.Close()) }()
		// The source keeps its first flag true, so each other factor replaces d.
		for i, power := range m.powers {
			if i == j {
				continue
			}
			if err = y.SetConstant(n, power.Variable); err != nil {
				return err
			}
			if err = yk.Power(y, power.Degree); err != nil {
				return err
			}
			if err = d.SetFrom(yk); err != nil {
				return err
			}
		}
		if err = y.SetConstant(n, m.variable); err != nil {
			return err
		}
		if err = r.Div(y, d); err != nil {
			return err
		}
	} else {
		if err = y.SetConstant(n, m.variable); err != nil {
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
	return c.propagateMonomialInterval(n, m.powers[j].Variable, m.variable, r)
}
