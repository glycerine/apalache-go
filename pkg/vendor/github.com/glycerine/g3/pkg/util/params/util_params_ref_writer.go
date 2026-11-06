package params

// Source: src/util/params.cpp:503-543, direct params_ref ostream displays.

import (
	"errors"
	"io"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// displayTo holds the context lock while source insertions traverse the live
// reference. A refused insertion sets the writer's sticky failbit, but the
// traversal continues and can report a later source error.
func (c *ParamsContext) displayTo(out io.Writer, v *ParamsRef, key *usymbol.Symbol, module *string, d *ParamDescrs) error {
	if c == nil || out == nil {
		return ErrParamsPrecondition
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	w := &sourceTextWriter{dst: out}
	c.formatWriter = w
	defer func() { c.formatWriter = nil }()
	if d != nil {
		if e := c.check(v); e != nil {
			return e
		}
		// params_ref::display_smt2 returns before inspecting the descriptor
		// pointer when the reference is empty.
		if v.Slot == 0 || c.Blocks[v.Slot-1].Size == 0 {
			return nil
		}
		d.mu.Lock()
		defer d.mu.Unlock()
	}
	return errors.Join(c.format(v, key, module, d), w.failure)
}

func (c *ParamsContext) DisplayTo(out io.Writer, v *ParamsRef) error {
	return c.displayTo(out, v, nil, nil, nil)
}

func (c *ParamsContext) DisplayKeyTo(out io.Writer, v *ParamsRef, key usymbol.Symbol) error {
	return c.displayTo(out, v, &key, nil, nil)
}

func (c *ParamsContext) DisplaySMT2To(out io.Writer, v *ParamsRef, module *string, d *ParamDescrs) error {
	if d == nil {
		// The empty-reference branch must still bypass this precondition.
		if c == nil || out == nil {
			return ErrParamsPrecondition
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		if e := c.check(v); e != nil {
			return e
		}
		if v.Slot == 0 || c.Blocks[v.Slot-1].Size == 0 {
			return nil
		}
		return ErrParamsPrecondition
	}
	return c.displayTo(out, v, nil, module, d)
}
