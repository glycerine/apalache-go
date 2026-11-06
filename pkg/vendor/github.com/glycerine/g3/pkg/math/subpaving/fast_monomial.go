package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:651-664,690-721,1499-1663,
// concrete config_mpff monomial definitions and propagation.

import (
	"errors"
	"math"
	"sort"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type FastPower struct {
	Variable uint32
	Degree   uint32
}

type FastMonomial struct {
	powers  []FastPower
	visited uint64
}

// DefineMonomial merges equal sorted variables and creates one output variable.
// The source marks the output integer when any factor variable is integer.
func (c *FastContext) DefineMonomial(powers []FastPower) (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	if len(powers) == 0 || len(c.isInt) >= math.MaxUint32 || (c.rootInitStarted || (c.root != nil && c.root.children != 0)) {
		return 0, arena.ErrInvalidSlot
	}
	merged := append([]FastPower(nil), powers...)
	for _, power := range merged {
		if power.Variable >= uint32(len(c.isInt)) {
			return 0, arena.ErrInvalidSlot
		}
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].Variable < merged[j].Variable })
	write := 0
	for read := 1; read < len(merged); read++ {
		if merged[write].Variable == merged[read].Variable {
			// C++ stores degrees as unsigned and merges with unsigned addition.
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
	if c.root != nil {
		c.root.lower = append(c.root.lower, nil)
		c.root.upper = append(c.root.upper, nil)
	}
	c.monomials[x] = &FastMonomial{powers: merged}
	for _, power := range merged {
		c.watchers[power.Variable] = append(c.watchers[power.Variable], x)
		c.watchOrder[power.Variable] = append(c.watchOrder[power.Variable], fastWatch{definition: x})
	}
	return x, nil
}

func (c *FastContext) PropagateMonomial(variable uint32, n *FastNode) (err error) {
	if err = c.checkNode(n, variable); err != nil {
		return err
	}
	if n.inconsistent {
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
		unbounded, e := n.IsUnbounded(power.Variable)
		if e != nil {
			return e
		}
		foundUnbounded = foundUnbounded || unbounded
	}
	if err = c.traceMonomialFlags(foundZero, foundUnbounded); err != nil {
		return err
	}
	if foundZero {
		zero, e := n.IsZero(variable)
		if e != nil {
			return e
		}
		if zero {
			return nil
		}
		owner, e := numeral.NewScopedFastFloat(c.numbers)
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, owner.Close()) }()
		v, e := owner.Get()
		if e != nil {
			return e
		}
		if err = c.numbers.SetInt32(v, 0); err != nil {
			return err
		}
		reason := fastBoundReason{kind: fastReasonVariableDefinition, variable: variable}
		if _, err = c.propagateCandidateReason(n, variable, v, true, false, reason); err != nil || n.inconsistent {
			return err
		}
		_, err = c.propagateCandidateReason(n, variable, v, false, false, reason)
		return err
	}
	xUnbounded, err := n.IsUnbounded(variable)
	if err != nil {
		return err
	}
	if !foundUnbounded {
		if err = c.propagateMonomialUpward(m, variable, n); err != nil || n.inconsistent {
			return err
		}
	}
	if xUnbounded {
		return nil
	}
	aux, err := NewFastInterval(c.numbers)
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
		if n.inconsistent {
			return nil
		}
		if err = c.propagateMonomialDownward(m, variable, i, n); err != nil {
			return err
		}
	}
	return nil
}

func (c *FastContext) propagateMonomialUpward(m *FastMonomial, variable uint32, n *FastNode) (err error) {
	r, err := NewFastInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, r.Close()) }()
	y, err := NewFastInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, y.Close()) }()
	yk, err := NewFastInterval(c.numbers)
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
	return c.propagateFastIntervalBounds(n, variable, variable, r)
}

func (c *FastContext) propagateMonomialDownward(m *FastMonomial, variable uint32, j int, n *FastNode) (err error) {
	if err = c.traceMonomialDownward(variable, j); err != nil {
		return err
	}
	r, err := NewFastInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, r.Close()) }()
	y, err := NewFastInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, y.Close()) }()
	if len(m.powers) > 1 {
		d, e := NewFastInterval(c.numbers)
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, d.Close()) }()
		first := true // The C++ source does not clear this flag in the loop.
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
	return c.propagateFastIntervalBounds(n, m.powers[j].Variable, variable, r)
}

func (c *FastContext) propagateFastIntervalBounds(n *FastNode, variable, sourceVariable uint32, r *FastInterval) error {
	lower, upper, err := r.endpoints()
	if err != nil {
		return err
	}
	if !r.lowerInf {
		if _, err := c.propagateCandidateReason(n, variable, lower, true, r.lowerOpen, fastBoundReason{kind: fastReasonVariableDefinition, variable: sourceVariable}); err != nil {
			return err
		}
		if n.inconsistent {
			return nil
		}
	}
	if !r.upperInf {
		_, err := c.propagateCandidateReason(n, variable, upper, false, r.upperOpen, fastBoundReason{kind: fastReasonVariableDefinition, variable: sourceVariable})
		return err
	}
	return nil
}
