package lp

// Source: src/util/lp/indexed_vector.h/.hpp numerical call ownership.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/numeral"
)

var ErrExactVectorPending = errors.New("lp: exact vector has a stopped numeral invocation")
var ErrExactVectorOwner = errors.New("lp: exact vectors belong to different numeral managers")

// These receipts record actual numeric calls made by concrete exact vector
// storage and its matrix/queue owners. This is not an arithmetic adapter:
// each algorithm calls the numeral/pair operations directly.
type exactVectorCalls struct {
	roots numeral.NumericAllocationRoots
	raw   [3]uint64
}
type exactVectorBefore struct {
	roots numeral.NumericAllocationRoots
	raw   numeral.RationalRawInvocationState
}

func (c *exactVectorCalls) begin(m *numeral.RationalManager) (exactVectorBefore, error) {
	if c.pending() {
		return exactVectorBefore{}, ErrExactVectorPending
	}
	roots, err := numeral.CaptureNumericAllocationRoots(m.Z)
	return exactVectorBefore{roots: roots, raw: m.RawInvocationState()}, err
}
func (c *exactVectorCalls) after(m *numeral.RationalManager, before exactVectorBefore, cause error) error {
	now := m.RawInvocationState()
	for i, active := range now.Active {
		if active && (!before.raw.Active[i] || now.Generations[i] != before.raw.Generations[i]) {
			c.raw[i] = now.Generations[i]
		}
	}
	return numeral.JoinNumericErrors(cause, c.roots.After(m.Z, before.roots))
}
func (c *exactVectorCalls) pending() bool {
	return c.raw != [3]uint64{} || numeral.NumericAllocationPending(&c.roots)
}
func (c *exactVectorCalls) cleanupRaw(m *numeral.RationalManager) error {
	return m.CleanupRawInvocations(&c.raw)
}
func (c *exactVectorCalls) cleanupRoots(m *numeral.RationalManager) error {
	return numeral.CleanupNumericAllocationRoots(m.Z, &c.roots)
}
