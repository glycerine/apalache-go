package lp

// Source: src/util/lp/binary_heap_priority_queue.hpp rational comparisons.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/numeral"
)

var ErrExactQueuePending = errors.New("lp: exact priority queue has a stopped numeral invocation")

// Concrete call receipts for the comparison group newly reached by priority
// queues, plus the existing numeric allocation-root receipt. No numeric value
// registry or arithmetic callback is used to decide ownership.
type exactPriorityCalls struct {
	numeric    exactVectorCalls
	comparison uint64
}
type exactPriorityBefore struct {
	numeric    exactVectorBefore
	comparison numeral.RationalComparisonInvocationState
}

func (c *exactPriorityCalls) pending() bool { return c.comparison != 0 || c.numeric.pending() }
func (c *exactPriorityCalls) begin(m *numeral.RationalManager) (exactPriorityBefore, error) {
	if c.pending() {
		return exactPriorityBefore{}, ErrExactQueuePending
	}
	before, err := c.numeric.begin(m)
	return exactPriorityBefore{numeric: before, comparison: m.ComparisonInvocationState()}, err
}
func (c *exactPriorityCalls) after(m *numeral.RationalManager, before exactPriorityBefore, cause error) error {
	now := m.ComparisonInvocationState()
	if now.Active && (!before.comparison.Active || now.Generation != before.comparison.Generation) {
		c.comparison = now.Generation
	}
	return c.numeric.after(m, before.numeric, cause)
}
func (c *exactPriorityCalls) cleanupRaw(m *numeral.RationalManager) error {
	if err := m.CleanupComparisonInvocation(&c.comparison); err != nil {
		return err
	}
	return c.numeric.cleanupRaw(m)
}
func (c *exactPriorityCalls) cleanupRoots(m *numeral.RationalManager) error {
	return c.numeric.cleanupRoots(m)
}
