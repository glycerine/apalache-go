package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:528-589,1337-1343,
// concrete config_hwf bound normalization, publication, and conflict check.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

var ErrHardwareContextTimestamp error = &sourceFailure{message: "subpaving: hardware bound timestamp exhausted"}

func (c *HardwareContext) checkNode(n *HardwareNode, variable uint32) error {
	if err := c.open(); err != nil {
		return err
	}
	if n == nil || n.context != c || variable >= uint32(len(c.isInt)) {
		return arena.ErrInvalidSlot
	}
	return n.open()
}

// NormalizeBound applies the source's ceil/floor and strict integer-bound
// adjustment to a context-owned scratch value.
func (c *HardwareContext) NormalizeBound(variable uint32, value *numeral.HardwareFloat, lower bool, open *bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if variable >= uint32(len(c.isInt)) || value == nil || open == nil {
		return arena.ErrInvalidSlot
	}
	m := c.numbers
	if err := m.Set(value, value); err != nil {
		return err
	}
	if !c.isInt[variable] {
		return nil
	}
	if !m.IsInteger(value) {
		*open = false
	}
	if lower {
		if err := m.Ceil(value, value); err != nil {
			return err
		}
	} else {
		if err := m.Floor(value, value); err != nil {
			return err
		}
	}
	if !*open {
		return nil
	}
	*open = false
	m.SetRounding(!lower)
	if lower {
		return m.Inc(value)
	}
	return m.Dec(value)
}

// ConflictingBounds follows the source's strict-order test before its
// conditional equal-value test for open endpoints.
func (n *HardwareNode) ConflictingBounds(variable uint32) (bool, error) {
	lower, err := n.bound(variable, true)
	if err != nil {
		return false, err
	}
	upper, err := n.bound(variable, false)
	if err != nil {
		return false, err
	}
	if lower == nil || upper == nil {
		return false, nil
	}
	if n.numbers.Less(&upper.value, &lower.value) {
		return true, nil
	}
	return (lower.open || upper.open) && n.numbers.Equal(&upper.value, &lower.value), nil
}

// PropagateBound creates a source mk_bound, then queues it for later
// propagation. Candidate relevance is checked by its eventual caller.
func (c *HardwareContext) PropagateBound(n *HardwareNode, variable uint32, value *numeral.HardwareFloat, lower, open bool) error {
	return c.propagateBoundReason(n, variable, value, lower, open, hardwareReasonUnknown, 0)
}

func (c *HardwareContext) propagateBoundReason(n *HardwareNode, variable uint32, value *numeral.HardwareFloat, lower, open bool, reason hardwareBoundReasonKind, reasonVariable uint32) error {
	return c.publishBound(n, variable, value, lower, open, reason, reasonVariable, true)
}

// DecideBound is the source splitter's unqueued mk_decided_bound path.
// AddRecentBounds queues surviving child decisions when that child is selected.
func (c *HardwareContext) DecideBound(n *HardwareNode, variable uint32, value *numeral.HardwareFloat, lower, open bool) error {
	if n == nil || n.parent == nil {
		return arena.ErrInvalidSlot
	}
	return c.publishBound(n, variable, value, lower, open, hardwareReasonAxiom, 0, false)
}

func (c *HardwareContext) publishBound(n *HardwareNode, variable uint32, value *numeral.HardwareFloat, lower, open bool, reason hardwareBoundReasonKind, reasonVariable uint32, enqueue bool) error {
	if err := c.checkNode(n, variable); err != nil {
		return err
	}
	if n.conflictVar != math.MaxUint32 {
		return arena.ErrInvalidSlot
	}
	var adjusted numeral.HardwareFloat
	if err := c.numbers.Set(&adjusted, value); err != nil {
		return err
	}
	if err := c.NormalizeBound(variable, &adjusted, lower, &open); err != nil {
		return err
	}
	c.numBounds++
	if err := n.PushBound(variable, &adjusted, lower, open); err != nil {
		return err
	}
	b := n.trail
	b.timestamp = c.timestamp
	b.reason = reason
	b.reasonVariable = reasonVariable
	if err := c.traceNewBound(b); err != nil {
		return err
	}
	conflict, err := n.ConflictingBounds(variable)
	if err != nil {
		return err
	}
	if conflict {
		if err := c.traceBoundConflict(n); err != nil {
			return err
		}
		c.numConflicts++
		n.conflictVar = variable
	}
	c.timestamp++
	if c.timestamp == math.MaxUint64 {
		return ErrHardwareContextTimestamp
	}
	if enqueue {
		c.queue = append(c.queue, b)
	}
	return nil
}
