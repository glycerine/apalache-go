package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:802-875,1773-1786,
// concrete config_hwf retained unit inequalities and root assertion.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/messages"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

var ErrHardwareContextCanceled = z3exception.NewDefaultException(messages.Canceled)

type hardwareUnitClause struct {
	atom  *HardwareInequality
	axiom bool
}

// AddUnitClause retains the same inequality pointer and axiom tag as the
// source's tagged unit vector.
func (c *HardwareContext) AddUnitClause(atom *HardwareInequality, axiom bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if atom == nil || atom.context != c || atom.variable >= uint32(len(c.isInt)) || atom.refs >= (1<<30)-1 {
		return arena.ErrInvalidSlot
	}
	if err := c.traceUnitInequality(atom); err != nil {
		return err
	}
	c.traceInequalityIncRef(atom)
	atom.refs++
	c.unitClauses = append(c.unitClauses, hardwareUnitClause{atom: atom, axiom: axiom})
	return nil
}

// AssertUnitClauses resumes at the first unasserted unit after cancellation.
// The source stops as soon as a published bound makes the root inconsistent.
func (c *HardwareContext) AssertUnitClauses(n *HardwareNode) error {
	if err := c.open(); err != nil {
		return err
	}
	if n == nil || n != c.root || n.context != c {
		return arena.ErrInvalidSlot
	}
	if err := n.open(); err != nil {
		return err
	}
	for c.unitHead < len(c.unitClauses) && n.conflictVar == ^uint32(0) {
		if err := sourceCheckpoint(c.limit, nil, c.process, c.maxMemory, ErrHardwareContextCanceled, c.cooperation); err != nil {
			return err
		}
		unit := c.unitClauses[c.unitHead]
		if err := c.traceInitialUnit(unit.atom, unit.axiom); err != nil {
			return err
		}
		reason := hardwareReasonAssumption
		if unit.axiom {
			reason = hardwareReasonAxiom
		}
		a := unit.atom
		if err := c.propagateBoundReason(n, a.variable, &a.value, a.lower, a.open, reason, 0); err != nil {
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
