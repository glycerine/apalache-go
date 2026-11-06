package params

// Source: src/util/params.cpp:682-690,753-763,805-828.

const paramsResetKey uint8 = 3

// ParamsResetKeyReceipt is minted only for the matched source entry. An absent
// key cannot acquire authority over an earlier stopped leaf.
type ParamsResetKeyReceipt struct {
	owner, invocation       uint64
	slot, generation, index uint32
}

func (c *ParamsContext) continueResetKey(slot uint32) error {
	b := &c.Blocks[slot-1]
	r := &c.decrementReceipts[slot-1]
	if !b.Live || !b.resetActive || b.resetKind != paramsResetKey || r.generation != b.Generation || r.invocation == 0 || b.resetCursor >= b.Size {
		return ErrParamsOwner
	}
	x := c.Entry(b, b.resetCursor)
	if b.resetStage == 0 {
		if x.Kind == ParamNumeral {
			if e := c.NumeralsField.Del(&x.Rat); e != nil {
				if r.cause == nil {
					r.cause = e
				}
				return e
			}
			c.traceRationalEntryDeallocated(x.ratTraceID)
		}
		b.resetStage = 1
	}
	// Explicit owned-string adapter release; source char strings are borrowed.
	if b.resetStage == 1 {
		if x.text.present && !x.text.borrowed {
			x.text.owned.release()
			x.text.present = false
		}
		b.resetStage = 2
	}
	// Source ascending assignment shifts the suffix before pop_back. There is
	// no fallible allocation in these typed entry assignments.
	for j := b.resetCursor + 1; j < b.Size; j++ {
		*c.Entry(b, j-1) = *c.Entry(b, j)
	}
	b.Size--
	*c.Entry(b, b.Size) = paramEntry{}
	b.resetActive = false
	b.resetKind = 0
	b.resetStage = 0
	b.resetCursor = 0
	return nil
}

func (c *ParamsContext) CompleteResetKey(v *ParamsRef, receipt ParamsResetKeyReceipt) error {
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
	if !b.resetActive || b.resetKind != paramsResetKey || receipt.index != b.resetCursor || r.generation != v.Generation || r.invocation != receipt.invocation || r.cause == nil {
		return ErrParamsPrecondition
	}
	return c.continueResetKey(v.Slot)
}
