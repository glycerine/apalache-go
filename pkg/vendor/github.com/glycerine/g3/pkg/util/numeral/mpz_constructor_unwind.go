package numeral

import (
	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// Failed construction destroys completed members in reverse declaration order.
// mpz/raw pointer fields do not own destructors; no six-cell del body runs.

func (m *IntegerManager) unwindConstructorMembers() error {
	for m.constructorUnwindStage < 2 {
		if m.constructorUnwindStage == 0 {
			if m.Natural != nil {
				if e := m.Natural.CloseChecked(); e != nil {
					return e
				}
				m.Natural = nil
			}
		} else if m.Allocator != nil {
			e := m.Allocator.Close()
			if m.Cells != nil {
				if mark := m.markSourceFreedPools(); mark != nil {
					return mark
				}
			}
			if e != nil {
				return e
			}
		}
		m.constructorUnwindStage++
	}
	m.ConstructorUnwound = true
	m.CleanupCauseField = nil
	return nil
}
func (m *IntegerManager) hasConstructorPhysicalRoots() bool {
	if m.PendingCell.Object.OwnerID() != 0 {
		if m.PendingCell.Offset == 0 {
			return true
		}
		// A saved pooled token is not physical storage after member chunk free.
		_, e := m.Memory.ReadUint32(m.PendingCell.Raw, m.PendingCell.Offset+4)
		if e != umemory.ErrRuntimeOwner {
			return true
		}
	}
	if m.Cells != nil {
		for n := uint64(1); n <= uint64(m.Cells.Stats().Slots); n++ {
			c, e := m.Cells.Read(arena.Slot(n))
			if e == nil && !c.Released {
				return true
			}
		}
	}
	return false
}
