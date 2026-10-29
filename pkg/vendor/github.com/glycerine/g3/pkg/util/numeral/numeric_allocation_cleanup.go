package numeral

import (
	"errors"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func CleanupNumericAllocationRoots(z *IntegerManager, r *NumericAllocationRoots) error {
	if r.Cell.Object != (umemory.ObjectRef{}) {
		old := r.Cell
		if z.PendingCell == old {
			e := z.cleanupPendingAllocation()
			if z.PendingCell != old {
				r.Cell = SourceCellBacking{}
			}
			if e != nil {
				return e
			}
		} else {
			r.Cell = SourceCellBacking{}
		}
	}
	now, e := z.Allocator.PendingRoots()
	if e != nil {
		return e
	}
	owned, foreign := false, false
	for i, x := range now {
		if x != (umemory.Block{}) {
			if x == r.Allocator[i] {
				owned = true
			} else {
				foreign = true
			}
		}
	}
	if owned && foreign {
		return ErrNumericCleanupPending
	}
	if owned {
		e = z.Allocator.CleanupPending()
		now, x := z.Allocator.PendingRoots()
		if x != nil {
			return errors.Join(e, x)
		}
		for i, old := range r.Allocator {
			if now[i] != old {
				r.Allocator[i] = umemory.Block{}
			}
		}
		if e != nil {
			return e
		}
	} else {
		for i, old := range r.Allocator {
			if now[i] != old {
				r.Allocator[i] = umemory.Block{}
			}
		}
	}
	return nil
}
