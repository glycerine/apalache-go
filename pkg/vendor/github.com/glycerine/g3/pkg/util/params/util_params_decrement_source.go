package params

// Source: src/util/params.cpp:325-339,510-513.

import (
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// Cold error metadata is prepared with the block directory. Actual leaf owners
// remain in their entries; this receipt never manufactures an owning numeral.
type paramsDecrementReceipt struct {
	cause      error
	invocation uint64
	generation uint32
}

func paramsReceiptBytes(n int) uint64 {
	return paramsBytes(n, reflect.TypeOf(paramsDecrementReceipt{}))
}

// continueDecrement is an explicit Go owning continuation, never a second
// source dec_ref. Its scalar cursor names the exact retained entry/leaf token.
func (c *ParamsContext) continueDecrement(slot uint32) error {
	if c.Blocks[slot-1].resetKind == paramsResetKey {
		return c.continueResetKey(slot)
	}
	if c.Blocks[slot-1].resetKind == paramsResetLive {
		return c.continueReset(slot)
	}
	b := &c.Blocks[slot-1]
	receipt := &c.decrementReceipts[slot-1]
	if !b.Live || !b.resetActive || receipt.generation != b.Generation {
		return ErrParamsOwner
	}
	for b.resetCursor < b.Size {
		entry := c.Entry(b, b.resetCursor)
		if b.resetStage == 0 {
			if entry.Kind == ParamNumeral {
				if e := c.NumeralsField.Del(&entry.Rat); e != nil {
					if receipt.cause == nil {
						receipt.cause = e
					}
					return e
				}
				c.traceRationalEntryDeallocated(entry.ratTraceID)
			}
			b.resetStage = 1
		}
		if b.resetStage == 1 {
			if entry.text.present && !entry.text.borrowed {
				entry.text.owned.release()
			}
			b.resetStage = 2
		}
		*entry = paramEntry{}
		b.resetCursor++
		b.resetStage = 0
	}
	c.freeEntries(b.entries)
	b.entries = paramsEntriesRef{}
	b.Size = 0
	b.capacity = 0
	if e := sourceAssertParamsResetEmpty(b); e != nil {
		if receipt.cause == nil {
			receipt.cause = e
		}
		return e
	}
	c.traceBlockDeallocated(slot)
	b.Live = false
	b.resetActive = false
	return nil
}

// CleanupDecrement acknowledges only the exact zero-reference block whose
// source destruction stopped. It retains its original cause in cold metadata
// until block reuse, and consumes the caller token only after all leaves finish.
func (c *ParamsContext) CleanupDecrement(v *ParamsRef) error {
	if c == nil || c.Self != c {
		return ErrParamsOwner
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.check(v); e != nil {
		return e
	}
	if v.Slot == 0 {
		return ErrParamsPrecondition
	}
	b := &c.Blocks[v.Slot-1]
	if !b.resetActive || b.resetKind != paramsResetDestroy || b.Refs != 0 {
		return ErrParamsPrecondition
	}
	if e := c.continueDecrement(v.Slot); e != nil {
		return e
	}
	*v = ParamsRef{}
	return nil
}

// A stopped destructor no longer denotes a valid ordinary source params value.
// Mutations cannot replace or resurrect its exact retained leaf owner. Reads
// retain their reached source guards; dec_ref retains its own DEBUG assertion.
func (c *ParamsContext) mutable(v *ParamsRef) error {
	if e := c.check(v); e != nil {
		return e
	}
	if v.Slot != 0 && (c.Blocks[v.Slot-1].resetActive || c.SourceFrames[v.Slot-1].Active || c.SetFrames[v.Slot-1].Active || c.replacementFrames[v.Slot-1].active) {
		return ErrParamsPrecondition
	}
	return nil
}

// ParamsDecrementReceipt binds a parent to the exact reached child destruction.
// It carries no authority derived from a block's reference count or registry.
type ParamsDecrementReceipt struct {
	owner, InvocationField uint64
	slot, generation       uint32
}

// CompleteDecrement consumes only a matching stopped invocation. It never
// repeats dec_ref, and leaves the original source failure in cold history.
func (c *ParamsContext) CompleteDecrement(v *ParamsRef, receipt ParamsDecrementReceipt) error {
	if c == nil || c.Self != c {
		return ErrParamsOwner
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.check(v); e != nil {
		return e
	}
	if v.Slot == 0 || receipt.owner != c.Owner || receipt.slot != v.Slot || receipt.generation != v.Generation || receipt.InvocationField == 0 {
		return ErrParamsPrecondition
	}
	b := &c.Blocks[v.Slot-1]
	r := &c.decrementReceipts[v.Slot-1]
	if !b.resetActive || b.resetKind != paramsResetDestroy || b.Refs != 0 || r.generation != v.Generation || r.invocation != receipt.InvocationField {
		return ErrParamsPrecondition
	}
	if e := c.continueDecrement(v.Slot); e != nil {
		return e
	}
	*v = ParamsRef{}
	return nil
}

func (c *ParamsContext) nextSourceInvocation() (uint64, error) {
	if c.sourceSerial == math.MaxUint64 {
		return 0, arena.ErrBounds
	}
	c.sourceSerial++
	return c.sourceSerial, nil
}

func (r ParamsDecrementReceipt) Invocation() uint64 { return r.InvocationField }
