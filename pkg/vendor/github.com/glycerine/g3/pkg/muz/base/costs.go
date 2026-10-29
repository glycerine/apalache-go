package base

// Source: src/muz/base/dl_costs.h and src/muz/base/dl_costs.cpp.

import (
	"errors"
	"fmt"
	"io"
	"time"
)

var ErrCosts = errors.New("muz/base: invalid cost accounting operation")

// Costs records the source instruction count and elapsed whole milliseconds.
type Costs struct {
	Milliseconds uint32
	Instructions uint32
}

func (c Costs) Empty() bool {
	return c.Milliseconds == 0 && c.Instructions == 0
}

func (c *Costs) Reset() {
	c.Milliseconds = 0
	c.Instructions = 0
}

func (c Costs) Subtract(other Costs) (Costs, error) {
	if c.Milliseconds <= other.Milliseconds || c.Instructions <= other.Instructions {
		return Costs{}, ErrCosts
	}
	return Costs{Milliseconds: c.Milliseconds - other.Milliseconds, Instructions: c.Instructions - other.Instructions}, nil
}

func (c *Costs) Add(other Costs) {
	c.Milliseconds += other.Milliseconds
	c.Instructions += other.Instructions
}

func (c Costs) PassesThreshold(milliseconds uint32) bool {
	return c.Milliseconds >= milliseconds
}

func (c Costs) Output(out io.Writer) error {
	if out == nil {
		return io.ErrClosedPipe
	}
	_, err := fmt.Fprintf(out, "instr: %d  time: %dms", c.Instructions, c.Milliseconds)
	return err
}

// AccountedObject is embedded by concrete relational instructions. A parent
// rule is retained while it receives propagated processed costs.
type AccountedObject struct {
	current       Costs
	processed     Costs
	parent        *AccountedObject
	parentRule    *Rule
	beingRecorded bool
	closed        bool
}

func NewAccountedObject() *AccountedObject {
	return &AccountedObject{}
}

func (o *AccountedObject) open() error {
	if o == nil || o.closed {
		return ErrCosts
	}
	return nil
}

func (o *AccountedObject) SetAccountingParentRule(parent *Rule) error {
	if err := o.open(); err != nil {
		return err
	}
	if parent == nil {
		return ErrCosts
	}
	state, err := parent.open()
	if err != nil || state.accounting == nil || state.accounting == o {
		return errors.Join(ErrCosts, err)
	}
	retained, err := parent.Retain()
	if err != nil {
		return err
	}
	if o.parentRule != nil {
		if err = o.parentRule.Close(); err != nil {
			return errors.Join(err, retained.Close())
		}
	}
	o.parentRule = retained
	o.parent = state.accounting
	return nil
}

func (o *AccountedObject) ParentRule() (*Rule, error) {
	if err := o.open(); err != nil {
		return nil, err
	}
	return o.parentRule, nil
}

func (o *AccountedObject) CurrentCosts() (*Costs, error) {
	if err := o.open(); err != nil {
		return nil, err
	}
	return &o.current, nil
}

func (o *AccountedObject) ProcessedCosts() (Costs, error) {
	if err := o.open(); err != nil {
		return Costs{}, err
	}
	return o.processed, nil
}

func (o *AccountedObject) TotalCosts() (Costs, error) {
	if err := o.open(); err != nil {
		return Costs{}, err
	}
	result := o.current
	result.Add(o.processed)
	return result, nil
}

func (o *AccountedObject) BeingRecorded() bool {
	return o != nil && !o.closed && o.beingRecorded
}

func (o *AccountedObject) ProcessCosts() error {
	if err := o.open(); err != nil {
		return err
	}
	delta := o.current
	if delta.Empty() {
		return nil
	}
	o.current.Reset()
	for object := o; object != nil; object = object.parent {
		object.processed.Add(delta)
	}
	return nil
}

func (o *AccountedObject) PassesOutputThreshold(milliseconds uint32) (bool, error) {
	total, err := o.TotalCosts()
	return total.PassesThreshold(milliseconds), err
}

func (o *AccountedObject) OutputProfile(out io.Writer) error {
	total, err := o.TotalCosts()
	if err != nil {
		return err
	}
	return total.Output(out)
}

func (o *AccountedObject) Close() error {
	if o == nil {
		return ErrCosts
	}
	if o.closed {
		return nil
	}
	if o.beingRecorded {
		return ErrCosts
	}
	o.closed = true
	if o.parentRule == nil {
		return nil
	}
	parent := o.parentRule
	o.parentRule = nil
	o.parent = nil
	return parent.Close()
}

// CostRecorder charges each completed recording interval as one instruction.
type CostRecorder struct {
	object   *AccountedObject
	now      func() time.Duration
	lastTime time.Duration
	closed   bool
}

func NewCostRecorder() *CostRecorder {
	start := time.Now()
	return newCostRecorder(func() time.Duration { return time.Since(start) })
}

func newCostRecorder(now func() time.Duration) *CostRecorder {
	return &CostRecorder{now: now}
}

func (r *CostRecorder) Start(object *AccountedObject) error {
	if r == nil || r.closed || r.now == nil {
		return ErrCosts
	}
	if object != nil {
		if err := object.open(); err != nil {
			return err
		}
		if object.beingRecorded && object != r.object {
			return ErrCosts
		}
	}
	current := r.now()
	if r.object != nil {
		if err := r.object.open(); err != nil {
			return err
		}
		delta := current - r.lastTime
		if delta < 0 {
			return ErrCosts
		}
		r.object.current.Instructions++
		r.object.current.Milliseconds += uint32(delta / time.Millisecond)
		r.object.beingRecorded = false
	}
	if object != nil {
		object.beingRecorded = true
	}
	r.object = object
	r.lastTime = current
	return nil
}

func (r *CostRecorder) Finish() error {
	return r.Start(nil)
}

func (r *CostRecorder) Close() error {
	if r == nil {
		return ErrCosts
	}
	if r.closed {
		return nil
	}
	if err := r.Finish(); err != nil {
		return err
	}
	r.closed = true
	return nil
}
