package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:773-869, concrete config_mpf
// inequality references, ordinary watched clauses and axiom units.

import (
	"math"
	"sort"

	"github.com/glycerine/g3/pkg/arena"
)

type ArbitraryClause struct {
	context   *ArbitraryContext
	atoms     []*ArbitraryInequality
	visited   uint64
	releaseAt int
}

type arbitraryTruth uint8

const (
	arbitraryUnknown arbitraryTruth = iota
	arbitraryTrue
	arbitraryFalse
)

// inequalityValue follows the source's upper/lower probe order and open-tie
// rules. An unbounded side leaves the atom unknown.
func (c *ArbitraryContext) inequalityValue(a *ArbitraryInequality, n *ArbitraryNode) (arbitraryTruth, error) {
	if err := c.open(); err != nil {
		return arbitraryUnknown, err
	}
	if err := c.checkInequality(a); err != nil {
		return arbitraryUnknown, err
	}
	if err := c.checkNode(n, a.variable); err != nil {
		return arbitraryUnknown, err
	}
	u, l := n.upper[a.variable], n.lower[a.variable]
	if u == nil && l == nil {
		return arbitraryUnknown, nil
	}
	m := c.numbers
	if a.lower {
		if u != nil {
			less, err := m.Less(&u.value, &a.value)
			if err != nil {
				return arbitraryUnknown, err
			}
			if less {
				return arbitraryFalse, nil
			}
			if u.open || a.open {
				equal, err := m.Equal(&u.value, &a.value)
				if err != nil {
					return arbitraryUnknown, err
				}
				if equal {
					return arbitraryFalse, nil
				}
			}
		}
		if l != nil {
			greater, err := m.Greater(&l.value, &a.value)
			if err != nil {
				return arbitraryUnknown, err
			}
			if greater {
				return arbitraryTrue, nil
			}
			if l.open || !a.open {
				equal, err := m.Equal(&l.value, &a.value)
				if err != nil {
					return arbitraryUnknown, err
				}
				if equal {
					return arbitraryTrue, nil
				}
			}
		}
	} else {
		if l != nil {
			greater, err := m.Greater(&l.value, &a.value)
			if err != nil {
				return arbitraryUnknown, err
			}
			if greater {
				return arbitraryFalse, nil
			}
			if l.open || a.open {
				equal, err := m.Equal(&l.value, &a.value)
				if err != nil {
					return arbitraryUnknown, err
				}
				if equal {
					return arbitraryFalse, nil
				}
			}
		}
		if u != nil {
			less, err := m.Less(&u.value, &a.value)
			if err != nil {
				return arbitraryUnknown, err
			}
			if less {
				return arbitraryTrue, nil
			}
			if u.open || !a.open {
				equal, err := m.Equal(&u.value, &a.value)
				if err != nil {
					return arbitraryUnknown, err
				}
				if equal {
					return arbitraryTrue, nil
				}
			}
		}
	}
	return arbitraryUnknown, nil
}

// PropagateClause publishes the sole unknown atom. With all atoms false it
// publishes the first one, exposing the ordinary bound conflict.
func (c *ArbitraryContext) PropagateClause(clause *ArbitraryClause, n *ArbitraryNode) error {
	if err := c.open(); err != nil {
		return err
	}
	if clause == nil || clause.context != c || len(clause.atoms) == 0 || n == nil || n.conflictVar != math.MaxUint32 {
		return arena.ErrInvalidSlot
	}
	if err := c.validNode(n); err != nil {
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
		case arbitraryTrue:
			return nil
		case arbitraryUnknown:
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
	if err := c.publishBoundReason(n, a.variable, &a.value, a.lower, a.open, false, clause); err != nil {
		return err
	}
	clause.visited = c.timestamp
	return nil
}

type arbitraryUnitClause struct {
	atom  *ArbitraryInequality
	axiom bool
}

func (c *ArbitraryContext) checkInequality(a *ArbitraryInequality) error {
	if a == nil || a.context != c || a.variable >= uint32(len(c.variables)) {
		return arena.ErrInvalidSlot
	}
	return nil
}

// AddUnitClause keeps an inequality alive for root unit assertion.
func (c *ArbitraryContext) AddUnitClause(a *ArbitraryInequality, axiom bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if err := c.checkInequality(a); err != nil {
		return err
	}
	if a.refs == math.MaxUint32 {
		return arena.ErrInvalidSlot
	}
	if err := c.traceUnitInequality(a); err != nil {
		return err
	}
	c.traceInequalityIncRef(a)
	a.refs++
	c.unitClauses = append(c.unitClauses, arbitraryUnitClause{atom: a, axiom: axiom})
	return nil
}

// AddClause follows the wrapper's non-lemma watched-clause route. A singleton
// is an axiom unit; longer clauses retain input order within each variable.
func (c *ArbitraryContext) AddClause(atoms []*ArbitraryInequality) (*ArbitraryClause, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if len(atoms) == 0 {
		return nil, arena.ErrInvalidSlot
	}
	counts := make(map[*ArbitraryInequality]uint32, len(atoms))
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
	ordered := append([]*ArbitraryInequality(nil), atoms...)
	clause := &ArbitraryClause{context: c, atoms: ordered}
	for _, a := range ordered {
		c.traceInequalityIncRef(a)
		a.refs++
	}
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].variable < ordered[j].variable })
	for i, a := range ordered {
		if i == 0 || a.variable != ordered[i-1].variable {
			c.clauseWatchers[a.variable] = append(c.clauseWatchers[a.variable], clause)
			c.watchOrder[a.variable] = append(c.watchOrder[a.variable], arbitraryWatch{clause: clause})
		}
	}
	c.clauses = append(c.clauses, clause)
	return clause, c.traceNewClause(clause)
}
