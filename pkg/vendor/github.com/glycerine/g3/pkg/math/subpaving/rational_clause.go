package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:773-869,1345-1419,
// concrete config_mpq watched clauses and exact inequality truth.

import (
	"math"
	"sort"

	"github.com/glycerine/g3/pkg/arena"
)

type RationalClause struct {
	context   *RationalContext
	atoms     []*RationalInequality
	visited   uint64
	releaseAt int
}

type rationalTruth uint8

const (
	rationalUnknown rationalTruth = iota
	rationalTrue
	rationalFalse
)

func (c *RationalContext) checkInequality(a *RationalInequality) error {
	if a == nil || a.context != c || a.closed || a.variable >= uint32(len(c.variables)) {
		return arena.ErrInvalidSlot
	}
	return nil
}

// AddClause retains input order for equal-variable atoms after sorting by
// variable. Each distinct variable gets one watch for this clause.
func (c *RationalContext) AddClause(atoms []*RationalInequality) (*RationalClause, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if len(atoms) == 0 {
		return nil, arena.ErrInvalidSlot
	}
	counts := make(map[*RationalInequality]uint32, len(atoms))
	for _, a := range atoms {
		if err := c.checkInequality(a); err != nil {
			return nil, err
		}
		counts[a]++
		if a.refs > math.MaxUint32-counts[a] {
			return nil, arena.ErrInvalidSlot
		}
	}
	if len(atoms) == 1 {
		return nil, c.AddUnitClause(atoms[0], true)
	}
	ordered := append([]*RationalInequality(nil), atoms...)
	clause := &RationalClause{context: c, atoms: ordered}
	for _, a := range ordered {
		c.traceInequalityIncRef(a)
		a.refs++
	}
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].variable < ordered[j].variable })
	for i, a := range ordered {
		if i == 0 || a.variable != ordered[i-1].variable {
			c.clauseWatchers[a.variable] = append(c.clauseWatchers[a.variable], clause)
			c.watchOrder[a.variable] = append(c.watchOrder[a.variable], rationalWatch{clause: clause})
		}
	}
	c.clauses = append(c.clauses, clause)
	if err := c.traceNewClause(clause); err != nil {
		return clause, err
	}
	return clause, nil
}

func (c *RationalContext) inequalityValue(a *RationalInequality, n *RationalNode) (rationalTruth, error) {
	if err := c.open(); err != nil {
		return rationalUnknown, err
	}
	if err := c.checkInequality(a); err != nil {
		return rationalUnknown, err
	}
	if n == nil || n.context != c || !n.registered {
		return rationalUnknown, arena.ErrInvalidSlot
	}
	u, l := n.upper[a.variable], n.lower[a.variable]
	if u == nil && l == nil {
		return rationalUnknown, nil
	}
	if a.lower {
		if u != nil {
			cmp, err := c.numbers.Compare(&u.value, &a.value)
			if err != nil {
				return rationalUnknown, err
			}
			if cmp < 0 || cmp == 0 && (u.open || a.open) {
				return rationalFalse, nil
			}
		}
		if l != nil {
			cmp, err := c.numbers.Compare(&l.value, &a.value)
			if err != nil {
				return rationalUnknown, err
			}
			if cmp > 0 || cmp == 0 && (l.open || !a.open) {
				return rationalTrue, nil
			}
		}
	} else {
		if l != nil {
			cmp, err := c.numbers.Compare(&l.value, &a.value)
			if err != nil {
				return rationalUnknown, err
			}
			if cmp > 0 || cmp == 0 && (l.open || a.open) {
				return rationalFalse, nil
			}
		}
		if u != nil {
			cmp, err := c.numbers.Compare(&u.value, &a.value)
			if err != nil {
				return rationalUnknown, err
			}
			if cmp < 0 || cmp == 0 && (u.open || !a.open) {
				return rationalTrue, nil
			}
		}
	}
	return rationalUnknown, nil
}

// PropagateClause publishes the sole unknown atom, or the first atom when all
// atoms are false so that ordinary bound conflict detection records the clash.
func (c *RationalContext) PropagateClause(clause *RationalClause, n *RationalNode) error {
	if err := c.open(); err != nil {
		return err
	}
	if clause == nil || clause.context != c || len(clause.atoms) < 2 || n == nil || n.context != c || !n.registered || n.conflictVar != math.MaxUint32 {
		return arena.ErrInvalidSlot
	}
	if err := n.open(); err != nil {
		return err
	}
	if err := c.tracePropagateClause(clause); err != nil {
		return err
	}
	c.numVisited++
	clause.visited = c.timestamp
	unknown := -1
	for i, a := range clause.atoms {
		truth, err := c.inequalityValue(a, n)
		if err != nil {
			return err
		}
		switch truth {
		case rationalTrue:
			return nil
		case rationalUnknown:
			if unknown >= 0 {
				return nil
			}
			unknown = i
		}
	}
	if unknown < 0 {
		unknown = 0
	}
	a := clause.atoms[unknown]
	if err := c.tracePropagatingInequality(a); err != nil {
		return err
	}
	if err := c.publishBoundReason(n, a.variable, &a.value, a.lower, a.open, false, clause, math.MaxUint32); err != nil {
		return err
	}
	c.queue = append(c.queue, n.trail)
	clause.visited = c.timestamp
	return nil
}
