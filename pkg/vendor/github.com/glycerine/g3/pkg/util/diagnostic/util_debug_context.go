package diagnostic

// Source: src/util/debug.cpp:28-79; src/util/debug.h:28-29,60-65,98.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// SourceAssertionsEnabled evaluates the selected source assertion profile and
// an optional borrowed debug context. It is shared by concrete checked ports.
func SourceAssertionsEnabled(d *DebugContext) (bool, error) {
	if !sourceDebug {
		return false, nil
	}
	if d == nil {
		return true, nil
	}
	return d.AssertionsEnabled()
}

var ErrDebugContextOwner = errors.New("g3: invalid debug context owner")

// DebugContext owns enabled debug names as immutable Go strings.
type DebugContext struct {
	Self             *DebugContext
	BudgetField      *arena.Budget
	Tags             map[string]struct{}
	region           *arena.BytesRegion
	assertionStreams *MemoryStandardStreams
	assertionActions *DebugActionContext
	memoryTrace      *TraceTags
	assertions       bool
	ownerBytes       uint64
	closed           bool
}

func NewDebugContext(region *arena.BytesRegion, b *arena.Budget) (*DebugContext, error) {
	if region == nil {
		return nil, arena.ErrConfig
	}
	n := uint64(reflect.TypeOf(DebugContext{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	d := &DebugContext{BudgetField: b, region: region, assertions: true, ownerBytes: n}
	d.Self = d
	return d, nil
}
func (d *DebugContext) open() error {
	if d == nil || d.Self != d {
		return ErrDebugContextOwner
	}
	if d.closed {
		return arena.ErrClosed
	}
	return nil
}
func (d *DebugContext) EnableAssertions(enabled bool) error {
	if e := d.open(); e != nil {
		return e
	}
	d.assertions = enabled
	return nil
}
func (d *DebugContext) AssertionsEnabled() (bool, error) {
	if e := d.open(); e != nil {
		return false, e
	}
	return d.assertions, nil
}

func debugTagName(tag string) string { return tag[:utext.CStringLength(tag)] }

func (d *DebugContext) EnableDebug(tag string) error {
	if e := d.open(); e != nil {
		return e
	}
	if d.Tags == nil {
		fresh := make(map[string]struct{})
		d.traceDebugTableAllocated()
		d.Tags = fresh
	}
	d.Tags[debugTagName(tag)] = struct{}{}
	return nil
}
func (d *DebugContext) DisableDebug(tag string) error {
	if e := d.open(); e != nil {
		return e
	}
	if d.Tags == nil {
		fresh := make(map[string]struct{})
		d.traceDebugTableAllocated()
		d.Tags = fresh
	}
	delete(d.Tags, debugTagName(tag))
	return nil
}
func (d *DebugContext) IsDebugEnabled(tag string) (bool, error) {
	if e := d.open(); e != nil {
		return false, e
	}
	if d.Tags == nil {
		fresh := make(map[string]struct{})
		d.traceDebugTableAllocated()
		d.Tags = fresh
	}
	_, ok := d.Tags[debugTagName(tag)]
	return ok, nil
}
func (d *DebugContext) IsDebugEnabledCString(tag string) (bool, error) {
	return d.IsDebugEnabled(tag)
}
func (d *DebugContext) FinalizeDebug() error {
	if e := d.open(); e != nil {
		return e
	}
	if d.Tags == nil {
		return nil
	}
	d.Tags = nil
	d.traceDebugTableDeallocated()
	return nil
}
func (d *DebugContext) Close() error {
	if d == nil || d.Self != d {
		return ErrDebugContextOwner
	}
	if d.closed {
		return nil
	}
	d.Tags = nil
	d.BudgetField.Release(d.ownerBytes)
	d.ownerBytes = 0
	d.closed = true
	return nil
}
func (d *DebugContext) ConditionalAssertionEnabled(tag string) (bool, error) {
	enabled, e := d.AssertionsEnabled()
	if e != nil || !enabled {
		return false, e
	}
	return d.IsDebugEnabled(tag)
}

func (d *DebugContext) Budget() *arena.Budget {
	if d == nil {
		return nil
	}
	return d.BudgetField
}
func (d *DebugContext) ValidOwner() bool { return d != nil && d.Self == d }
