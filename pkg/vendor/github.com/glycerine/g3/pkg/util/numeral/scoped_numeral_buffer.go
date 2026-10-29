package numeral

// Source: src/util/scoped_numeral_buffer.h; src/util/buffer.h:29-59,146-151,210-242.

import "github.com/glycerine/g3/pkg/arena"

// scopedIntegerModularBuffer128 is the concrete polynomial mpzzp
// specialization. Ordinary Go slice storage replaces the source inline/spill
// buffer. Each Int remains an explicitly owned numeral and is copied or
// destroyed through IntegerModularManager in source order.
type scopedIntegerModularBuffer128 struct {
	manager                *IntegerModularManager
	values                 []Int
	numeric                NumericAllocationRoots
	bodyCause, sourceCause error
	Constructed, closed    bool
}

func (b *scopedIntegerModularBuffer128) construct(m *IntegerModularManager) {
	*b = scopedIntegerModularBuffer128{manager: m, Constructed: true}
}
func (b *scopedIntegerModularBuffer128) open() error {
	if !b.Constructed || b.closed {
		return arena.ErrClosed
	}
	return b.manager.check()
}
func (b *scopedIntegerModularBuffer128) physical(i uint32) (Int, error) {
	if uint64(i) >= uint64(len(b.values)) {
		return Int{}, arena.ErrBounds
	}
	return b.values[i], nil
}
func (b *scopedIntegerModularBuffer128) get(i uint32) (Int, error) {
	if e := b.open(); e != nil {
		return Int{}, e
	}
	if modularSourceDebug && uint64(i) >= uint64(len(b.values)) {
		return Int{}, ErrNumericAssertion
	}
	return b.physical(i)
}
func (b *scopedIntegerModularBuffer128) store(i uint32, v Int) error {
	if uint64(i) >= uint64(len(b.values)) {
		return arena.ErrBounds
	}
	b.values[i] = v
	return nil
}
func (b *scopedIntegerModularBuffer128) pushZero() error {
	if uint64(len(b.values)) == uint64(^uint32(0)) {
		return arena.ErrBounds
	}
	b.values = append(b.values, Int{})
	return nil
}
func (b *scopedIntegerModularBuffer128) set(i uint32, v *Int) (err error) {
	if e := b.open(); e != nil {
		return e
	}
	if b.sourceCause != nil {
		return b.sourceCause
	}
	if b.bodyCause != nil {
		return b.bodyCause
	}
	if modularSourceDebug && uint64(i) >= uint64(len(b.values)) {
		return ErrNumericAssertion
	}
	a, e := b.physical(i)
	if e != nil {
		return e
	}
	before, e := CaptureNumericAllocationRoots(b.manager.manager)
	if e != nil {
		return e
	}
	err = b.manager.Copy(&a, v)
	after := b.numeric.After(b.manager.manager, before)
	stored := b.store(i, a)
	err = JoinNumericErrors(err, JoinNumericErrors(after, stored))
	if err != nil && b.bodyCause == nil {
		b.bodyCause = err
	}
	return err
}
func (b *scopedIntegerModularBuffer128) push(v *Int) (err error) {
	if e := b.open(); e != nil {
		return e
	}
	if b.sourceCause != nil {
		return b.sourceCause
	}
	if b.bodyCause != nil {
		return b.bodyCause
	}
	if e := b.pushZero(); e != nil {
		b.bodyCause = e
		return e
	}
	return b.set(uint32(len(b.values)-1), v)
}
func (b *scopedIntegerModularBuffer128) del(i uint32, owning bool) error {
	a, e := b.physical(i)
	if e != nil {
		return e
	}
	if owning {
		e = b.manager.Del(&a)
	} else {
		_, e = SourceNumeralDel(b.manager.manager, &a)
	}
	return JoinNumericErrors(e, b.store(i, a))
}
func (b *scopedIntegerModularBuffer128) shrink(n uint32) error {
	if e := b.open(); e != nil {
		return e
	}
	if b.sourceCause != nil {
		return b.sourceCause
	}
	if modularSourceDebug && uint64(n) > uint64(len(b.values)) {
		return ErrNumericAssertion
	}
	if uint64(n) >= uint64(len(b.values)) {
		return nil
	}
	for i := n; uint64(i) < uint64(len(b.values)); i++ {
		if e := b.del(i, false); e != nil {
			b.sourceCause = e
			return e
		}
	}
	clear(b.values[n:])
	b.values = b.values[:n]
	return nil
}
func (b *scopedIntegerModularBuffer128) reset() error { return b.shrink(0) }
func (b *scopedIntegerModularBuffer128) Resize(n uint32) error {
	if e := b.open(); e != nil {
		return e
	}
	if b.sourceCause != nil {
		return b.sourceCause
	}
	if b.bodyCause != nil {
		return b.bodyCause
	}
	if uint64(n) <= uint64(len(b.values)) {
		return b.shrink(n)
	}
	if uint64(n) > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	old := len(b.values)
	b.values = append(b.values, make([]Int, int(n)-old)...)
	return nil
}
func (b *scopedIntegerModularBuffer128) sourceClose() error {
	if !b.Constructed || b.closed {
		return nil
	}
	if b.sourceCause != nil {
		return b.sourceCause
	}
	if e := b.reset(); e != nil {
		return e
	}
	b.values = nil
	b.closed = true
	return nil
}
func (b *scopedIntegerModularBuffer128) cleanup() error {
	if !b.Constructed {
		return nil
	}
	if e := b.numeric.Cleanup(b.manager.manager); e != nil {
		return e
	}
	for i := range b.values {
		if e := b.del(uint32(i), true); e != nil {
			return e
		}
	}
	b.values = nil
	b.closed = true
	return nil
}

// view borrows the current Go slice. It does not clone numerals or acquire
// ownership.
func (b *scopedIntegerModularBuffer128) view() (IntegerArrayView, error) {
	if e := b.open(); e != nil {
		return IntegerArrayView{}, e
	}
	return IntegerArrayView{Values: b.values, Count: uint32(len(b.values))}, nil
}
