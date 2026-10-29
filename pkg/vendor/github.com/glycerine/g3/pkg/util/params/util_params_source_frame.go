package params

// Source: src/util/params.cpp:550-558,598-610.

import (
	"reflect"
)

const (
	paramsFrameAssign uint8 = 1
	paramsFrameInit   uint8 = 2
)

type paramsSourceFrame struct {
	cause                                    error
	Invocation                               uint64
	rootGeneration, HeldSlot, HeldGeneration uint32
	kind                                     uint8
	Active, decrementStarted                 bool
}

// ParamsSourceReceipt is a concrete binding to one failed source invocation.
// Fields are private; a caller obtains it only from its current actual value.
type ParamsSourceReceipt struct {
	owner, InvocationField uint64
	Slot, generation       uint32
}

func paramsSourceFrameBytes(n int) uint64 { return paramsBytes(n, reflect.TypeOf(paramsSourceFrame{})) }
func (c *ParamsContext) StoppedSource(v *ParamsRef) (ParamsSourceReceipt, error) {
	if c == nil || c.Self != c {
		return ParamsSourceReceipt{}, ErrParamsOwner
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.check(v); e != nil {
		return ParamsSourceReceipt{}, e
	}
	if v.Slot == 0 {
		return ParamsSourceReceipt{}, ErrParamsPrecondition
	}
	f := &c.SourceFrames[v.Slot-1]
	if !f.Active || f.rootGeneration != v.Generation || f.cause == nil {
		return ParamsSourceReceipt{}, ErrParamsPrecondition
	}
	return ParamsSourceReceipt{owner: c.Owner, InvocationField: f.Invocation, Slot: v.Slot, generation: v.Generation}, nil
}

// CleanupSource discards only the exact reference retained by a failed source
// frame. It does not replay copying, pointer publication, or a consumed DEC.
func (c *ParamsContext) CleanupSource(r ParamsSourceReceipt) error {
	if c == nil || c.Self != c {
		return ErrParamsOwner
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Closed || r.owner != c.Owner || r.Slot == 0 || int(r.Slot) > len(c.SourceFrames) {
		return ErrParamsOwner
	}
	f := &c.SourceFrames[r.Slot-1]
	if !f.Active || f.Invocation != r.InvocationField || f.rootGeneration != r.generation || r.InvocationField == 0 {
		return ErrParamsPrecondition
	}
	return c.cleanupSourceFrame(r.Slot)
}
func (c *ParamsContext) cleanupSourceFrame(slot uint32) error {
	f := &c.SourceFrames[slot-1]
	if !f.Active || f.Invocation == 0 || f.rootGeneration != c.Blocks[slot-1].Generation || (f.kind != paramsFrameAssign && f.kind != paramsFrameInit) || f.HeldSlot == 0 || int(f.HeldSlot) > len(c.Blocks) {
		return ErrParamsOwner
	}
	b := &c.Blocks[f.HeldSlot-1]
	if b.Generation != f.HeldGeneration || !b.Live {
		return ErrParamsOwner
	}
	if b.resetActive && (b.resetKind == paramsResetLive || b.resetKind == paramsResetKey) {
		if e := c.continueDecrement(f.HeldSlot); e != nil {
			return e
		}
	}
	if !f.decrementStarted {
		if b.Refs == 0 {
			return ErrParamsPrecondition
		}
		f.decrementStarted = true
		if e := c.dec(f.HeldSlot); e != nil {
			return e
		}
	} else {
		if !b.resetActive || b.Refs != 0 {
			return ErrParamsOwner
		}
		if e := c.continueDecrement(f.HeldSlot); e != nil {
			return e
		}
	}
	f.Active = false
	return nil
}

func (r ParamsSourceReceipt) Invocation() uint64 { return r.InvocationField }
