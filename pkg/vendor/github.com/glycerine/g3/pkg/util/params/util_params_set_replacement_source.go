package params

// Source: src/util/params.cpp:753-763,946-1014,1017-1033.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// leaf is a borrowed scalar identity receipt, never a second owning numeral.
// The sole old leaf remains in the actual matched entry. next owns only any
// prepared Go copied text; native SET_STR borrows a C-string instead.
type paramsReplacementFrame struct {
	cause, cleanupCause error
	invocation          uint64
	generation, index   uint32
	oldKind             ParamKind
	phase               uint8
	active              bool
	leaf                unumeral.OwnedRational
	next                paramEntry
}
type ParamsSetReplacementReceipt struct {
	owner, invocation       uint64
	slot, generation, index uint32
}

func paramsReplacementFrameBytes(n int) uint64 {
	return paramsBytes(n, reflect.TypeOf(paramsReplacementFrame{}))
}
func (c *ParamsContext) beginReplacement(slot, index uint32, next paramEntry, serial uint64) error {
	b := &c.Blocks[slot-1]
	if c.replacementFrames[slot-1].active || index >= b.Size {
		return ErrParamsPrecondition
	}
	entry := c.Entry(b, index)
	if entry.Kind != ParamNumeral {
		return ErrParamsPrecondition
	}
	if serial == 0 {
		return ErrParamsPrecondition
	}
	// Actual prepared text ownership transfers at this assignment; no root is
	// inferred from a refcount, allocation count, or descriptor registry.
	c.replacementFrames[slot-1] = paramsReplacementFrame{invocation: serial, generation: b.Generation, index: index, oldKind: entry.Kind, active: true, leaf: entry.Rat, next: next}
	return c.continueReplacement(slot, true)
}
func (c *ParamsContext) continueReplacement(slot uint32, publish bool) error {
	f := &c.replacementFrames[slot-1]
	b := &c.Blocks[slot-1]
	if !f.active || !b.Live || f.generation != b.Generation || f.invocation == 0 || f.index >= b.Size || f.oldKind != ParamNumeral {
		return ErrParamsOwner
	}
	entry := c.Entry(b, f.index)
	if f.phase == 0 {
		if entry.Kind != f.oldKind || entry.Rat != f.leaf {
			return ErrParamsPrecondition
		}
		e := c.NumeralsField.Del(&entry.Rat)
		f.leaf = entry.Rat // The real leaf call may consume only a completed prefix.
		if e != nil {
			if f.cause == nil {
				f.cause = e
			} else if f.cleanupCause == nil {
				f.cleanupCause = e
			}
			return e
		}
		c.traceRationalEntryDeallocated(entry.ratTraceID)
		f.phase = 1
	}
	if publish {
		if f.phase == 1 {
			// SET_SYM publishes the kind before reaching v.bare_str(). A
			// stopped assertion keeps this frame after consuming the old leaf.
			entry.Kind = f.next.Kind
			entry.bits = f.next.bits
			entry.Rat = f.next.Rat
			entry.ratTraceID = 0
			entry.text = f.next.text
			f.phase = 2
		}
		if f.next.Kind == ParamSymbol && f.next.symbol.IsNumerical() {
			spelling, _, e := c.SymbolsField.BareString(f.next.symbol)
			if e == nil {
				entry.symbol, e = c.SymbolsField.Intern(spelling)
			}
			if e != nil {
				if f.cause == nil {
					f.cause = e
				} else if f.cleanupCause == nil {
					f.cleanupCause = e
				}
				return e
			}
		} else {
			entry.symbol = f.next.symbol
		}
	} else {
		// Context owning abort is distinct from resuming the failed setter body.
		// It discards only the actual prepared argument and consumed old leaf.
		if f.next.text.present && !f.next.text.borrowed {
			f.next.text.owned.release()
		}
		*entry = paramEntry{}
	}
	f.next = paramEntry{}
	f.active = false
	f.phase = 2
	return nil
}
func (c *ParamsContext) StoppedSetReplacement(v *ParamsRef) (ParamsSetReplacementReceipt, error) {
	if c == nil || c.Self != c {
		return ParamsSetReplacementReceipt{}, ErrParamsOwner
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.check(v); e != nil {
		return ParamsSetReplacementReceipt{}, e
	}
	if v.Slot == 0 {
		return ParamsSetReplacementReceipt{}, ErrParamsPrecondition
	}
	f := &c.replacementFrames[v.Slot-1]
	if !f.active || f.cause == nil || f.generation != v.Generation {
		return ParamsSetReplacementReceipt{}, ErrParamsPrecondition
	}
	return ParamsSetReplacementReceipt{owner: c.Owner, slot: v.Slot, generation: v.Generation, index: f.index, invocation: f.invocation}, nil
}
func (c *ParamsContext) CompleteSetReplacement(v *ParamsRef, r ParamsSetReplacementReceipt) error {
	return c.finishSetReplacement(v, r, true)
}

// AbortSetReplacement finishes only the retained owning cleanup after a
// stopped setter or copy_core replacement. It does not publish the incoming
// value or resume the source assignment; the caller must release the affected
// ParamsRef after this cleanup.
func (c *ParamsContext) AbortSetReplacement(v *ParamsRef, r ParamsSetReplacementReceipt) error {
	return c.finishSetReplacement(v, r, false)
}

func (c *ParamsContext) finishSetReplacement(v *ParamsRef, r ParamsSetReplacementReceipt, publish bool) error {
	if c == nil || c.Self != c {
		return ErrParamsOwner
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.check(v); e != nil {
		return e
	}
	if v.Slot == 0 || r.owner != c.Owner || r.slot != v.Slot || r.generation != v.Generation || r.invocation == 0 {
		return ErrParamsPrecondition
	}
	f := &c.replacementFrames[v.Slot-1]
	if !f.active || f.invocation != r.invocation || f.index != r.index || f.generation != r.generation {
		return ErrParamsPrecondition
	}
	return c.continueReplacement(v.Slot, publish)
}
