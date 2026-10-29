package params

// Source: src/util/params.cpp:938-944,992-1014.

import (
	"reflect"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// Prepared cold storage retains the actual acquired new_entry value when
// copying or appending throws. The source POD local has no rational destructor.
type paramsSetFrame struct {
	Cause, CleanupCause error
	Invocation          uint64
	Generation          uint32
	Active, Acquired    bool
	Published           bool
	MatchedIndex        uint32
	Next                paramEntry
	Child               unumeral.RationalCopyAllocationReceipt
}
type ParamsSetReceipt struct {
	owner, InvocationField uint64
	Slot, Generation       uint32
}

func paramsSetFrameBytes(n int) uint64 { return paramsBytes(n, reflect.TypeOf(paramsSetFrame{})) }
func (c *ParamsContext) setNewNumeral(slot uint32, key usymbol.Symbol, value *unumeral.OwnedRational) error {
	frame := &c.SetFrames[slot-1]
	if frame.Active {
		return ErrParamsPrecondition
	}
	serial, e := c.nextSourceInvocation()
	if e != nil {
		return e
	}
	*frame = paramsSetFrame{Generation: c.Blocks[slot-1].Generation, Invocation: serial, Next: paramEntry{key: key, Kind: ParamNumeral, ratTraceID: serial}}
	r, e := c.NumeralsField.NewValue()
	if e != nil {
		frame.Cause = e
		return e
	}
	c.traceRationalEntryAllocated(serial, 1004)
	frame.Next.Rat = r
	frame.Acquired = true
	frame.Active = true
	if e = c.NumeralsField.CopyWithAllocationReceipt(&frame.Next.Rat, value, &frame.Child); e != nil {
		frame.Cause = e
		return e
	}
	b := &c.Blocks[slot-1]
	if b.Size == b.capacity {
		if e = c.grow(b); e != nil {
			frame.Cause = e
			return e
		}
	}
	*c.Entry(b, b.Size) = frame.Next
	b.Size++
	frame.Next = paramEntry{}
	frame.Child = unumeral.RationalCopyAllocationReceipt{}
	frame.Acquired = false
	frame.Active = false
	return nil
}

// Source SET_RAT MATCH deletes the old non-numeral value before allocating
// and copying the replacement rational at its original entry position.
func (c *ParamsContext) setMatchedNewNumeral(slot, index uint32, key usymbol.Symbol, value *unumeral.OwnedRational) error {
	frame := &c.SetFrames[slot-1]
	b := &c.Blocks[slot-1]
	if frame.Active || index >= b.Size {
		return ErrParamsPrecondition
	}
	entry := c.Entry(b, index)
	if entry.key != key || entry.Kind == ParamNumeral {
		return ErrParamsPrecondition
	}
	serial, e := c.nextSourceInvocation()
	if e != nil {
		return e
	}
	if e = c.delEntry(entry); e != nil {
		return e
	}
	r, e := c.NumeralsField.NewValue()
	if e != nil {
		return e
	}
	c.traceRationalEntryAllocated(serial, 996)
	*entry = paramEntry{key: key, Kind: ParamNumeral, Rat: r, ratTraceID: serial}
	*frame = paramsSetFrame{Invocation: serial, Generation: b.Generation, Active: true, Acquired: true, Published: true, MatchedIndex: index, Next: paramEntry{key: key, Kind: ParamNumeral, ratTraceID: serial}}
	if e = c.NumeralsField.CopyWithAllocationReceipt(&entry.Rat, value, &frame.Child); e != nil {
		frame.Cause = e
		return e
	}
	*frame = paramsSetFrame{}
	return nil
}
func (c *ParamsContext) StoppedSet(v *ParamsRef) (ParamsSetReceipt, error) {
	if c == nil || c.Self != c {
		return ParamsSetReceipt{}, ErrParamsOwner
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.check(v); e != nil {
		return ParamsSetReceipt{}, e
	}
	if v.Slot == 0 {
		return ParamsSetReceipt{}, ErrParamsPrecondition
	}
	f := &c.SetFrames[v.Slot-1]
	if !f.Active || !f.Acquired || f.Generation != v.Generation || f.Cause == nil {
		return ParamsSetReceipt{}, ErrParamsPrecondition
	}
	return ParamsSetReceipt{owner: c.Owner, Slot: v.Slot, Generation: v.Generation, InvocationField: f.Invocation}, nil
}
func (c *ParamsContext) CleanupSet(receipt ParamsSetReceipt) error {
	if c == nil || c.Self != c {
		return ErrParamsOwner
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Closed || receipt.owner != c.Owner || receipt.Slot == 0 || int(receipt.Slot) > len(c.SetFrames) {
		return ErrParamsOwner
	}
	f := &c.SetFrames[receipt.Slot-1]
	if !f.Active || !f.Acquired || f.Invocation != receipt.InvocationField || receipt.InvocationField == 0 || f.Generation != receipt.Generation {
		return ErrParamsPrecondition
	}
	return c.cleanupSetFrame(receipt.Slot)
}
func (c *ParamsContext) cleanupSetFrame(slot uint32) error {
	f := &c.SetFrames[slot-1]
	if !f.Active || !f.Acquired || f.Invocation == 0 || f.Generation != c.Blocks[slot-1].Generation || f.Next.Kind != ParamNumeral {
		return ErrParamsOwner
	}
	b := &c.Blocks[slot-1]
	var owned *unumeral.OwnedRational
	if f.Published {
		if f.MatchedIndex >= b.Size {
			return ErrParamsOwner
		}
		entry := c.Entry(b, f.MatchedIndex)
		if entry.key != f.Next.key || entry.Kind != ParamNumeral {
			return ErrParamsOwner
		}
		owned = &entry.Rat
	} else {
		owned = &f.Next.Rat
	}
	if f.Child.Active() {
		if e := c.NumeralsField.CleanupCopyAllocation(&f.Child); e != nil {
			if f.CleanupCause == nil {
				f.CleanupCause = e
			}
			return e
		}
	}
	f.Child = unumeral.RationalCopyAllocationReceipt{}
	if e := c.NumeralsField.Del(owned); e != nil {
		if f.CleanupCause == nil {
			f.CleanupCause = e
		}
		return e
	}
	c.traceRationalEntryDeallocated(f.Next.ratTraceID)
	if f.Published {
		for i := f.MatchedIndex + 1; i < b.Size; i++ {
			*c.Entry(b, i-1) = *c.Entry(b, i)
		}
		*c.Entry(b, b.Size-1) = paramEntry{}
		b.Size--
	}
	f.Next = paramEntry{}
	f.Active = false
	f.Acquired = false
	f.Published = false
	return nil
}

func (r ParamsSetReceipt) Invocation() uint64 { return r.InvocationField }
