package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:802-880,1349-1405,
// concrete config_mpff inequalities, clauses and unit assertions.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// FastInequality owns a copied fast-float value through its context. A clause
// may refer to the same inequality more than once, as in the source.
type FastInequality struct {
	context  *FastContext
	owner    *numeral.ScopedFastFloat
	value    *numeral.FastFloat
	variable uint32
	lower    bool
	open     bool
	refs     uint32
}

type FastClause struct {
	context        *FastContext
	atoms          []*FastInequality
	lemma          bool
	watched        bool
	visited        uint64
	releaseAt      int
	watchesRemoved bool
	deleted        bool
}

type fastUnitClause struct {
	atom     *FastInequality
	axiom    bool
	released bool
}

type fastWatch struct {
	definition uint32
	clause     *FastClause
}

func (c *FastContext) NewInequality(variable uint32, value *numeral.FastFloat, lower, open bool) (_ *FastInequality, err error) {
	if err = c.open(); err != nil {
		return nil, err
	}
	if variable >= uint32(len(c.isInt)) {
		return nil, arena.ErrInvalidSlot
	}
	owner, err := numeral.NewScopedFastFloat(c.numbers)
	if err != nil {
		return nil, err
	}
	v, err := owner.Get()
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	if err = c.numbers.Copy(v, value); err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	a := &FastInequality{context: c, owner: owner, value: v, variable: variable, lower: lower, open: open}
	c.inequalities = append(c.inequalities, a)
	return a, nil
}

func (c *FastContext) checkInequality(a *FastInequality) error {
	if a == nil || a.context != c || a.owner == nil || a.variable >= uint32(len(c.isInt)) {
		return arena.ErrInvalidSlot
	}
	_, err := a.owner.Get()
	return err
}

func (c *FastContext) releaseInequality(a *FastInequality) error {
	if a.refs > 0 {
		if err := c.traceInequalityDecRef(a); err != nil {
			return err
		}
		a.refs--
	}
	if a.refs == 0 {
		return a.owner.Close()
	}
	return nil
}

func (c *FastContext) releaseUnit(i int) error {
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

func (c *FastContext) releaseClause(clause *FastClause) error {
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
func (c *FastContext) AddClause(atoms []*FastInequality, lemma, watched bool) (*FastClause, error) {
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
	copyAtoms := append([]*FastInequality(nil), atoms...)
	clause := &FastClause{context: c, atoms: copyAtoms, lemma: lemma, watched: watched}
	for _, atom := range copyAtoms {
		c.traceInequalityIncRef(atom)
		atom.refs++
	}
	sort.SliceStable(copyAtoms, func(i, j int) bool { return copyAtoms[i].variable < copyAtoms[j].variable })
	if watched {
		var previous uint32
		for i, atom := range copyAtoms {
			if i == 0 || atom.variable != previous {
				c.watchOrder[atom.variable] = append(c.watchOrder[atom.variable], fastWatch{clause: clause})
			}
			previous = atom.variable
		}
	}
	c.clauses = append(c.clauses, clause)
	return clause, c.traceNewClause(clause)
}

func (c *FastContext) AddUnitClause(atom *FastInequality, axiom bool) error {
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
	c.unitClauses = append(c.unitClauses, fastUnitClause{atom: atom, axiom: axiom})
	return nil
}

// AssertUnitClauses is the source root-initialization pass before definition
// and queue propagation. A failed checkpoint keeps the remaining suffix.
func (c *FastContext) AssertUnitClauses(n *FastNode) error {
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
		if err := sourceCheckpoint(c.limit, c.runtime, c.process, c.maxMemory, ErrFastContextCanceled, c.cooperation); err != nil {
			return err
		}
		unit := c.unitClauses[c.unitHead]
		a := unit.atom
		if err := c.traceInitialUnit(a, unit.axiom); err != nil {
			return err
		}
		reason := fastBoundReason{kind: fastReasonAssumption}
		if unit.axiom {
			reason.kind = fastReasonAxiom
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

type fastTruth uint8

const (
	fastUnknown fastTruth = iota
	fastTrue
	fastFalse
)

func (c *FastContext) inequalityValue(a *FastInequality, n *FastNode) (fastTruth, error) {
	if err := c.open(); err != nil {
		return fastUnknown, err
	}
	if err := c.checkInequality(a); err != nil {
		return fastUnknown, err
	}
	if err := c.checkNode(n, a.variable); err != nil {
		return fastUnknown, err
	}
	u, l := n.upper[a.variable], n.lower[a.variable]
	if u == nil && l == nil {
		return fastUnknown, nil
	}
	cmp := func(b *FastBound) (int, error) { return fastCompare(c.numbers, b.value, a.value) }
	if a.lower {
		if u != nil {
			v, err := cmp(u)
			if err != nil {
				return fastUnknown, err
			}
			if v < 0 || v == 0 && (u.open || a.open) {
				return fastFalse, nil
			}
		}
		if l != nil {
			v, err := cmp(l)
			if err != nil {
				return fastUnknown, err
			}
			if v > 0 || v == 0 && (l.open || !a.open) {
				return fastTrue, nil
			}
		}
	} else {
		if l != nil {
			v, err := cmp(l)
			if err != nil {
				return fastUnknown, err
			}
			if v > 0 || v == 0 && (l.open || a.open) {
				return fastFalse, nil
			}
		}
		if u != nil {
			v, err := cmp(u)
			if err != nil {
				return fastUnknown, err
			}
			if v < 0 || v == 0 && (u.open || !a.open) {
				return fastTrue, nil
			}
		}
	}
	return fastUnknown, nil
}

// PropagateClause selects the sole unknown atom, or the first atom when all
// are false, and publishes its bound directly to expose source conflict.
func (c *FastContext) PropagateClause(clause *FastClause, n *FastNode) error {
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
		case fastTrue:
			return nil
		case fastUnknown:
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
	if err := c.propagateBoundReason(n, a.variable, a.value, a.lower, a.open, fastBoundReason{kind: fastReasonClause, clause: clause}); err != nil {
		return err
	}
	clause.visited = c.timestamp
	return nil
}
