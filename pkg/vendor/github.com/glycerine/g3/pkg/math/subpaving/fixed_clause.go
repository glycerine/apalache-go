package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:802-880,1349-1405,
// concrete config_mpfx inequalities, clauses and unit assertions.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// FixedInequality owns a copied fixed-point value through its context. A clause
// may refer to the same inequality more than once, as in the source.
type FixedInequality struct {
	context  *FixedContext
	value    *numeral.FixedPoint
	variable uint32
	lower    bool
	open     bool
	refs     uint32
}

type FixedClause struct {
	context        *FixedContext
	atoms          []*FixedInequality
	lemma          bool
	watched        bool
	visited        uint64
	releaseAt      int
	watchesRemoved bool
	deleted        bool
}

type fixedUnitClause struct {
	atom     *FixedInequality
	axiom    bool
	released bool
}

type fixedWatch struct {
	definition uint32
	clause     *FixedClause
}

func (c *FixedContext) NewInequality(variable uint32, value *numeral.FixedPoint, lower, open bool) (_ *FixedInequality, err error) {
	if err = c.open(); err != nil {
		return nil, err
	}
	if variable >= uint32(len(c.isInt)) {
		return nil, arena.ErrInvalidSlot
	}
	v, err := c.numbers.New()
	if err != nil {
		return nil, err
	}
	if err = c.numbers.Copy(v, value); err != nil {
		return nil, errors.Join(err, v.Close())
	}
	a := &FixedInequality{context: c, value: v, variable: variable, lower: lower, open: open}
	c.inequalities = append(c.inequalities, a)
	return a, nil
}

func (c *FixedContext) checkInequality(a *FixedInequality) error {
	if a == nil || a.context != c || a.value == nil || a.variable >= uint32(len(c.isInt)) {
		return arena.ErrInvalidSlot
	}
	_, err := c.numbers.IsZero(a.value)
	return err
}

func (c *FixedContext) releaseInequality(a *FixedInequality) error {
	if a.refs > 0 {
		if err := c.traceInequalityDecRef(a); err != nil {
			return err
		}
		a.refs--
	}
	if a.refs == 0 {
		return a.value.Close()
	}
	return nil
}

func (c *FixedContext) releaseUnit(i int) error {
	unit := &c.unitClauses[i]
	if unit.released {
		return nil
	}
	if err := c.releaseInequality(unit.atom); err != nil {
		return err
	}
	unit.released = true
	return nil
}

func (c *FixedContext) releaseClause(clause *FixedClause) error {
	if clause.deleted {
		return nil
	}
	if !clause.watchesRemoved {
		if clause.watched {
			var previous uint32
			for i, atom := range clause.atoms {
				if i != 0 && atom.variable == previous {
					continue
				}
				watchers := c.watchOrder[atom.variable]
				for j, watch := range watchers {
					if watch.clause == clause {
						c.watchOrder[atom.variable] = append(watchers[:j], watchers[j+1:]...)
						break
					}
				}
				previous = atom.variable
			}
		}
		clause.watchesRemoved = true
	}
	for clause.releaseAt < len(clause.atoms) {
		if err := c.releaseInequality(clause.atoms[clause.releaseAt]); err != nil {
			return err
		}
		clause.releaseAt++
	}
	clause.deleted = true
	return nil
}

// AddClause retains the input atom pointers in stable variable order. Source
// singleton clauses enter the unit list as axioms regardless of lemma flag.
func (c *FixedContext) AddClause(atoms []*FixedInequality, lemma, watched bool) (*FixedClause, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if len(atoms) == 0 || !lemma && !watched {
		return nil, arena.ErrInvalidSlot
	}
	for _, atom := range atoms {
		if err := c.checkInequality(atom); err != nil {
			return nil, err
		}
	}
	if len(atoms) == 1 {
		return nil, c.AddUnitClause(atoms[0], true)
	}
	copyAtoms := append([]*FixedInequality(nil), atoms...)
	clause := &FixedClause{context: c, atoms: copyAtoms, lemma: lemma, watched: watched}
	for _, atom := range copyAtoms {
		c.traceInequalityIncRef(atom)
		atom.refs++
	}
	sort.SliceStable(copyAtoms, func(i, j int) bool { return copyAtoms[i].variable < copyAtoms[j].variable })
	if watched {
		var previous uint32
		for i, atom := range copyAtoms {
			if i == 0 || atom.variable != previous {
				c.watchOrder[atom.variable] = append(c.watchOrder[atom.variable], fixedWatch{clause: clause})
			}
			previous = atom.variable
		}
	}
	c.clauses = append(c.clauses, clause)
	return clause, c.traceNewClause(clause)
}

func (c *FixedContext) AddUnitClause(atom *FixedInequality, axiom bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if err := c.checkInequality(atom); err != nil {
		return err
	}
	if err := c.traceUnitInequality(atom); err != nil {
		return err
	}
	c.traceInequalityIncRef(atom)
	atom.refs++
	c.unitClauses = append(c.unitClauses, fixedUnitClause{atom: atom, axiom: axiom})
	return nil
}

// AssertUnitClauses is the source root-initialization pass before definition
// and queue propagation. A failed checkpoint keeps the remaining suffix.
func (c *FixedContext) AssertUnitClauses(n *FixedNode) error {
	if err := c.open(); err != nil {
		return err
	}
	if n == nil || n.context != c || n != c.root {
		return arena.ErrInvalidSlot
	}
	if err := n.open(); err != nil {
		return err
	}
	for c.unitHead < len(c.unitClauses) && !n.inconsistent {
		if !c.limit.Inc() {
			return ErrFixedContextCanceled
		}
		unit := c.unitClauses[c.unitHead]
		a := unit.atom
		if err := c.traceInitialUnit(a, unit.axiom); err != nil {
			return err
		}
		reason := fixedBoundReason{kind: fixedReasonAssumption}
		if unit.axiom {
			reason.kind = fixedReasonAxiom
		}
		if err := c.propagateBoundReason(n, a.variable, a.value, a.lower, a.open, reason); err != nil {
			return err
		}
		c.unitHead++
	}
	if !c.initUnitBoundsTraced {
		if err := c.traceInitialBounds(n, "bounds after init\n", 1785); err != nil {
			return err
		}
		c.initUnitBoundsTraced = true
	}
	return nil
}

type fixedTruth uint8

const (
	fixedUnknown fixedTruth = iota
	fixedTrue
	fixedFalse
)

func (c *FixedContext) inequalityValue(a *FixedInequality, n *FixedNode) (fixedTruth, error) {
	if err := c.open(); err != nil {
		return fixedUnknown, err
	}
	if err := c.checkInequality(a); err != nil {
		return fixedUnknown, err
	}
	if err := c.checkNode(n, a.variable); err != nil {
		return fixedUnknown, err
	}
	u, l := n.upper[a.variable], n.lower[a.variable]
	if u == nil && l == nil {
		return fixedUnknown, nil
	}
	cmp := func(b *FixedBound) (int, error) { return c.numbers.Compare(b.value, a.value) }
	if a.lower {
		if u != nil {
			v, err := cmp(u)
			if err != nil {
				return fixedUnknown, err
			}
			if v < 0 || v == 0 && (u.open || a.open) {
				return fixedFalse, nil
			}
		}
		if l != nil {
			v, err := cmp(l)
			if err != nil {
				return fixedUnknown, err
			}
			if v > 0 || v == 0 && (l.open || !a.open) {
				return fixedTrue, nil
			}
		}
	} else {
		if l != nil {
			v, err := cmp(l)
			if err != nil {
				return fixedUnknown, err
			}
			if v > 0 || v == 0 && (l.open || a.open) {
				return fixedFalse, nil
			}
		}
		if u != nil {
			v, err := cmp(u)
			if err != nil {
				return fixedUnknown, err
			}
			if v < 0 || v == 0 && (u.open || !a.open) {
				return fixedTrue, nil
			}
		}
	}
	return fixedUnknown, nil
}

// PropagateClause selects the sole unknown atom, or the first atom when all
// are false, and publishes its bound directly to expose source conflict.
func (c *FixedContext) PropagateClause(clause *FixedClause, n *FixedNode) error {
	if err := c.open(); err != nil {
		return err
	}
	if clause == nil || clause.context != c || clause.deleted || n == nil || n.context != c || n.inconsistent {
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
	for i, atom := range clause.atoms {
		value, err := c.inequalityValue(atom, n)
		if err != nil {
			return err
		}
		switch value {
		case fixedTrue:
			return nil
		case fixedUnknown:
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
	if err := c.propagateBoundReason(n, a.variable, a.value, a.lower, a.open, fixedBoundReason{kind: fixedReasonClause, clause: clause}); err != nil {
		return err
	}
	clause.visited = c.timestamp
	return nil
}
