package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:802-875,1349-1405,
// concrete config_hwf clauses, inequality truth and unit propagation.

import (
	"sort"

	"github.com/glycerine/g3/pkg/arena"
)

// HardwareClause retains its sorted atom pointers and source watcher flags.
type HardwareClause struct {
	context   *HardwareContext
	atoms     []*HardwareInequality
	lemma     bool
	watched   bool
	visited   uint64
	deleted   bool
	releaseAt int
}

type hardwareWatch struct {
	definition uint32
	clause     *HardwareClause
}

func (c *HardwareContext) checkInequality(a *HardwareInequality) error {
	if a == nil || a.context != c || a.variable >= uint32(len(c.isInt)) {
		return arena.ErrInvalidSlot
	}
	return nil
}

// AddClause preserves stable variable order and registers one watcher per
// distinct variable. A singleton becomes an axiom unit in source order.
func (c *HardwareContext) AddClause(atoms []*HardwareInequality, lemma, watched bool) (*HardwareClause, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if len(atoms) == 0 || !lemma && !watched {
		return nil, arena.ErrInvalidSlot
	}
	counts := make(map[*HardwareInequality]uint32, len(atoms))
	for _, atom := range atoms {
		if err := c.checkInequality(atom); err != nil {
			return nil, err
		}
		counts[atom]++
		if atom.refs > (1<<30)-1-counts[atom] {
			return nil, arena.ErrInvalidSlot
		}
	}
	if len(atoms) == 1 {
		return nil, c.AddUnitClause(atoms[0], true)
	}
	ordered := append([]*HardwareInequality(nil), atoms...)
	clause := &HardwareClause{context: c, atoms: ordered, lemma: lemma, watched: watched}
	for _, atom := range ordered {
		c.traceInequalityIncRef(atom)
		atom.refs++
	}
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].variable < ordered[j].variable })
	if watched {
		var previous uint32
		for i, atom := range ordered {
			if i == 0 || atom.variable != previous {
				c.watchOrder[atom.variable] = append(c.watchOrder[atom.variable], hardwareWatch{clause: clause})
			}
			previous = atom.variable
		}
	}
	c.clauses = append(c.clauses, clause)
	return clause, c.traceNewClause(clause)
}

type hardwareTruth uint8

const (
	hardwareUnknown hardwareTruth = iota
	hardwareTrue
	hardwareFalse
)

func (c *HardwareContext) inequalityValue(a *HardwareInequality, n *HardwareNode) (hardwareTruth, error) {
	if err := c.open(); err != nil {
		return hardwareUnknown, err
	}
	if err := c.checkInequality(a); err != nil {
		return hardwareUnknown, err
	}
	if err := c.checkNode(n, a.variable); err != nil {
		return hardwareUnknown, err
	}
	m := c.numbers
	u, l := n.upper[a.variable], n.lower[a.variable]
	if u == nil && l == nil {
		return hardwareUnknown, nil
	}
	if a.lower {
		if u != nil && (m.Less(&u.value, &a.value) || (u.open || a.open) && m.Equal(&u.value, &a.value)) {
			return hardwareFalse, nil
		}
		if l != nil && (m.Greater(&l.value, &a.value) || (l.open || !a.open) && m.Equal(&l.value, &a.value)) {
			return hardwareTrue, nil
		}
	} else {
		if l != nil && (m.Greater(&l.value, &a.value) || (l.open || a.open) && m.Equal(&l.value, &a.value)) {
			return hardwareFalse, nil
		}
		if u != nil && (m.Less(&u.value, &a.value) || (u.open || !a.open) && m.Equal(&u.value, &a.value)) {
			return hardwareTrue, nil
		}
	}
	return hardwareUnknown, nil
}

// PropagateClause publishes the sole unknown atom. If every atom is false,
// the source publishes the first one to expose an ordinary bound conflict.
func (c *HardwareContext) PropagateClause(clause *HardwareClause, n *HardwareNode) error {
	if err := c.open(); err != nil {
		return err
	}
	if clause == nil || clause.context != c || clause.deleted || n == nil || n.context != c || n.conflictVar != ^uint32(0) {
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
		case hardwareTrue:
			return nil
		case hardwareUnknown:
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
	previous := n.trail
	err := c.propagateBoundReason(n, a.variable, &a.value, a.lower, a.open, hardwareReasonClause, 0)
	if n.trail != previous {
		n.trail.reasonClause = clause
	}
	if err != nil {
		return err
	}
	clause.visited = c.timestamp
	return nil
}
