package polynomial

import umemory "github.com/glycerine/g3/pkg/util/memory"

// Explicit owning acknowledgement of the cell allocation's exact unjoined raw
// storage. Ordinary source failures leave the allocator's pending state intact.
func (m *AlgebraicRootCore) cleanupCellAllocation() error {
	a := m.config.Allocator
	if m.cellPending.Block() == (umemory.Block{}) && a.MatchesPendingAllocationOutcome(&m.cellAllocation) {
		if e := a.TransferAllocationPending(&m.cellAllocation, &m.cellPending); e != nil {
			return e
		}
	}
	if m.cellPending.Block() != (umemory.Block{}) {
		e := a.CleanupTransferredAllocation(&m.cellPending)
		if m.cellPending.Block() == (umemory.Block{}) {
			m.cellPending = umemory.SmallAllocatorPendingAllocation{}
		}
		if e != nil {
			return e
		}
	}
	return nil
}
