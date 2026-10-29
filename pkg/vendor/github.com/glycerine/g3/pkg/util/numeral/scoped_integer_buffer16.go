package numeral

// Source: src/util/scoped_numeral_buffer.h:25-71; concrete unsynch_mpz_manager
// specialization in src/math/subpaving/tactic/expr2subpaving.cpp:221-263.

import "github.com/glycerine/g3/pkg/arena"

// ScopedIntegerBuffer16 is the two concrete integer buffers used by
// expr2subpaving::process_add. Its native slice owns copied Int values; At
// borrows an element until the next slice-changing operation.
type ScopedIntegerBuffer16 struct {
	manager                *IntegerManager
	values                 []Int
	numeric                NumericAllocationRoots
	bodyCause, sourceCause error
	constructed, closed    bool
}

func (b *ScopedIntegerBuffer16) Construct(m *IntegerManager) error {
	if b == nil || b.constructed || m == nil {
		return arena.ErrInvalidSlot
	}
	var zero Int
	if e := m.Check(&zero); e != nil {
		return e
	}
	*b = ScopedIntegerBuffer16{manager: m, constructed: true}
	return nil
}

func (b *ScopedIntegerBuffer16) check() error {
	if b == nil || !b.constructed || b.closed {
		return arena.ErrClosed
	}
	var zero Int
	return b.manager.Check(&zero)
}

func (b *ScopedIntegerBuffer16) Count() uint32 { return uint32(len(b.values)) }

func (b *ScopedIntegerBuffer16) At(i uint32) (*Int, error) {
	if e := b.check(); e != nil {
		return nil, e
	}
	if uint64(i) >= uint64(len(b.values)) {
		return nil, arena.ErrBounds
	}
	return &b.values[i], nil
}

func (b *ScopedIntegerBuffer16) View() (IntegerArrayView, error) {
	if e := b.check(); e != nil {
		return IntegerArrayView{}, e
	}
	return IntegerArrayView{Values: b.values, Count: uint32(len(b.values))}, nil
}

func (b *ScopedIntegerBuffer16) Push(v *Int) error {
	if e := b.check(); e != nil {
		return e
	}
	if b.sourceCause != nil {
		return b.sourceCause
	}
	if b.bodyCause != nil {
		return b.bodyCause
	}
	if uint64(len(b.values)) == uint64(^uint32(0)) {
		return arena.ErrBounds
	}
	// The source appends an initialized zero before the manager's deep set.
	b.values = append(b.values, Int{})
	at := len(b.values) - 1
	before, e := CaptureNumericAllocationRoots(b.manager)
	if e != nil {
		b.bodyCause = e
		return e
	}
	e = b.manager.Copy(&b.values[at], v)
	after := b.numeric.After(b.manager, before)
	e = JoinNumericErrors(e, after)
	if e != nil {
		b.bodyCause = e
	}
	return e
}

// MultiplyAt is the reached process_add numerator-scaling operation. It
// retains any allocation prefix produced by an interrupted in-place multiply.
func (b *ScopedIntegerBuffer16) MultiplyAt(i uint32, factor *Int) error {
	if e := b.check(); e != nil {
		return e
	}
	if b.sourceCause != nil {
		return b.sourceCause
	}
	if b.bodyCause != nil {
		return b.bodyCause
	}
	if uint64(i) >= uint64(len(b.values)) {
		return arena.ErrBounds
	}
	before, e := CaptureNumericAllocationRoots(b.manager)
	if e != nil {
		b.bodyCause = e
		return e
	}
	at := &b.values[i]
	e = b.manager.Mul(at, factor, at)
	after := b.numeric.After(b.manager, before)
	e = JoinNumericErrors(e, after)
	if e != nil {
		b.bodyCause = e
	}
	return e
}

func (b *ScopedIntegerBuffer16) sourceDelete(i int) error {
	a := b.values[i]
	_, e := SourceNumeralDel(b.manager, &a)
	b.values[i] = a
	return e
}

// SourceClose follows the destructor's ascending deletion order. A stopped
// source deletion leaves later numeral slots intact for explicit Cleanup.
func (b *ScopedIntegerBuffer16) SourceClose() error {
	if b == nil || !b.constructed || b.closed {
		return nil
	}
	if b.sourceCause != nil {
		return b.sourceCause
	}
	for i := range b.values {
		if e := b.sourceDelete(i); e != nil {
			b.sourceCause = e
			return e
		}
	}
	b.values = nil
	b.closed = true
	return nil
}

// Cleanup is the explicit owning continuation after a stopped source scope.
func (b *ScopedIntegerBuffer16) Cleanup() error {
	if b == nil || !b.constructed {
		return nil
	}
	if e := b.numeric.Cleanup(b.manager); e != nil {
		return e
	}
	for i := range b.values {
		if e := b.manager.Del(&b.values[i]); e != nil {
			return e
		}
	}
	b.values = nil
	b.closed = true
	return nil
}
