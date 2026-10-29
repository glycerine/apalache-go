package diagnostic

// Source: src/util/util.cpp:22-40; src/util/util.h:176-186.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	utext "github.com/glycerine/g3/pkg/util/text"
)

var ErrVerboseContextOwner = errors.New("g3: invalid verbosity context owner")

// VerboseContext is an explicit single-goroutine owner for the source globals.
// The initial stream is supplied by the caller; native stderr wiring is open.
// Stream selection borrows its pointer without reading the pointed object.
type VerboseContext struct {
	self       *VerboseContext
	budget     *arena.Budget
	stream     *utext.NumericTextStream
	ownerBytes uint64
	level      uint32
	closed     bool
}

func NewVerboseContext(stream *utext.NumericTextStream, b *arena.Budget) (*VerboseContext, error) {
	if stream == nil || b == nil {
		return nil, arena.ErrConfig
	}
	n := uint64(reflect.TypeOf(VerboseContext{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	c := &VerboseContext{budget: b, stream: stream, ownerBytes: n}
	c.self = c
	return c, nil
}
func (c *VerboseContext) open() error {
	if c == nil || c.self != c {
		return ErrVerboseContextOwner
	}
	if c.closed {
		return arena.ErrClosed
	}
	return nil
}
func (c *VerboseContext) SetVerbosityLevel(level uint32) error {
	if e := c.open(); e != nil {
		return e
	}
	c.level = level
	return nil
}
func (c *VerboseContext) VerbosityLevel() (uint32, error) {
	if e := c.open(); e != nil {
		return 0, e
	}
	return c.level, nil
}
func (c *VerboseContext) SetVerboseStream(stream *utext.NumericTextStream) error {
	if e := c.open(); e != nil {
		return e
	}
	if stream == nil {
		return arena.ErrConfig
	}
	c.stream = stream
	return nil
}
func (c *VerboseContext) VerboseStream() (*utext.NumericTextStream, error) {
	if e := c.open(); e != nil {
		return nil, e
	}
	return c.stream, nil
}
func (c *VerboseContext) Close() error {
	if c == nil || c.self != c {
		return ErrVerboseContextOwner
	}
	if c.closed {
		return nil
	}
	c.budget.Release(c.ownerBytes)
	c.ownerBytes = 0
	c.closed = true
	return nil
}
