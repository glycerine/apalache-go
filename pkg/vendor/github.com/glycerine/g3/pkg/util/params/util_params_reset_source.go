package params

// Source: src/util/params.cpp:677-680,753-781,799-803.

import udiag "github.com/glycerine/g3/pkg/util/diagnostic"

const (
	paramsResetDestroy uint8 = 1
	paramsResetLive    uint8 = 2
)

// ParamsResetReceipt binds one live-value reset invocation, without transferring
// the caller's reference or turning its shared block into a zero-ref destructor.
type ParamsResetReceipt struct {
	owner, invocation uint64
	slot, generation  uint32
}

func sourceAssertParamsResetEmpty(b *paramsBlock) error {
	if !udiag.DebugBuild {
		return nil
	}
	d := udiag.BorrowProcessDebugContext()
	enabled, e := udiag.SourceAssertionsEnabled(d)
	if e != nil || !enabled || b.Size == 0 {
		return e
	}
	return udiag.DebugAssertionFailure(d, "src/util/params.cpp", 802, "empty()")
}

func (c *ParamsContext) continueReset(slot uint32) error {
	b := &c.Blocks[slot-1]
	r := &c.decrementReceipts[slot-1]
	if !b.Live || !b.resetActive || b.resetKind != paramsResetLive || r.generation != b.Generation || r.invocation == 0 {
		return ErrParamsOwner
	}
	// del_values preserves the actual scalar entries until vector.finalize.
	if b.resetStage == 0 {
		for b.resetCursor < b.Size {
			x := c.Entry(b, b.resetCursor)
			if x.Kind == ParamNumeral {
				if e := c.NumeralsField.Del(&x.Rat); e != nil {
					if r.cause == nil {
						r.cause = e
					}
					return e
				}
				c.traceRationalEntryDeallocated(x.ratTraceID)
			}
			b.resetCursor++
		}
		b.resetStage = 1
		b.resetCursor = 0
	}
	// Owned copied text is a Go adapter. Its release belongs to final backing
	// disposal, after the source numeral traversal, not to inline-value deletion.
	for b.resetCursor < b.Size {
		x := c.Entry(b, b.resetCursor)
		if x.text.present && !x.text.borrowed {
			x.text.owned.release()
			x.text.present = false
		}
		b.resetCursor++
	}
	c.freeEntries(b.entries)
	b.entries = paramsEntriesRef{}
	b.Size = 0
	b.capacity = 0
	if e := sourceAssertParamsResetEmpty(b); e != nil {
		if r.cause == nil {
			r.cause = e
		}
		return e
	}
	b.resetActive = false
	b.resetCursor = 0
	b.resetStage = 0
	b.resetKind = 0
	return nil
}

// CompleteReset is an explicit owning continuation of the matching stopped
// action. It preserves caller identity and the live block's reference count.
func (c *ParamsContext) CompleteReset(v *ParamsRef, receipt ParamsResetReceipt) error {
	if c == nil || c.Self != c {
		return ErrParamsOwner
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.check(v); e != nil {
		return e
	}
	if v.Slot == 0 || receipt.owner != c.Owner || receipt.slot != v.Slot || receipt.generation != v.Generation || receipt.invocation == 0 {
		return ErrParamsPrecondition
	}
	b := &c.Blocks[v.Slot-1]
	r := &c.decrementReceipts[v.Slot-1]
	if !b.resetActive || b.resetKind != paramsResetLive || r.generation != v.Generation || r.invocation != receipt.invocation || r.cause == nil {
		return ErrParamsPrecondition
	}
	return c.continueReset(v.Slot)
}
