package numeral

// Source: src/util/scoped_numeral_buffer.h; src/util/buffer.h.

// ScopedIntegerModularBuffer128 is the concrete source scoped buffer. Its
// enclosing caller owns its lifetime; its native slice stores explicitly owned
// numerals whose copy and destruction stay in this utility implementation.
type ScopedIntegerModularBuffer128 = scopedIntegerModularBuffer128

func (b *scopedIntegerModularBuffer128) Construct(m *IntegerModularManager) { b.construct(m) }
func (b *scopedIntegerModularBuffer128) Check() error                       { return b.open() }
func (b *scopedIntegerModularBuffer128) Manager() *IntegerModularManager    { return b.manager }
func (b *scopedIntegerModularBuffer128) Count() uint32                      { return uint32(len(b.values)) }
func (b *scopedIntegerModularBuffer128) Physical(i uint32) (Int, error)     { return b.physical(i) }
func (b *scopedIntegerModularBuffer128) Get(i uint32) (Int, error)          { return b.get(i) }
func (b *scopedIntegerModularBuffer128) Store(i uint32, a Int) error        { return b.store(i, a) }
func (b *scopedIntegerModularBuffer128) PushZero() error                    { return b.pushZero() }
func (b *scopedIntegerModularBuffer128) Push(a *Int) error                  { return b.push(a) }
func (b *scopedIntegerModularBuffer128) Set(i uint32, a *Int) error         { return b.set(i, a) }
func (b *scopedIntegerModularBuffer128) View() (IntegerArrayView, error)    { return b.view() }
func (b *scopedIntegerModularBuffer128) SourceClose() error                 { return b.sourceClose() }
func (b *scopedIntegerModularBuffer128) Cleanup() error                     { return b.cleanup() }
func (b *scopedIntegerModularBuffer128) BodyCause() error                   { return b.bodyCause }
func (b *scopedIntegerModularBuffer128) SourceCause() error                 { return b.sourceCause }
func (b *scopedIntegerModularBuffer128) RecordBodyCause(e error) {
	if b.bodyCause == nil {
		b.bodyCause = e
	}
}
func (b *scopedIntegerModularBuffer128) AllocationRoots() *NumericAllocationRoots { return &b.numeric }
