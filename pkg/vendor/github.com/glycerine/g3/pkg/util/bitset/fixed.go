package bitset

// Translation of src/util/fixed_bit_vector.h and fixed_bit_vector.cpp.

import (
	"reflect"
	"strings"

	"github.com/glycerine/g3/pkg/arena"
)

// FixedRef is interpreted only within the owning FixedManager. It contains no
// pointers; source pointer identity becomes slot plus generation identity.
type FixedRef = arena.VectorRef

// FixedManager owns fixed-width vectors in typed, pointer-free pooled word
// pages. Copy/Clone are explicit; callers must not copy this owner struct.
// Concurrent access requires owner synchronization.
type FixedManager struct {
	words                             *arena.Words
	budget                            *arena.Budget
	rootBytes                         uint64
	numBits, numWords, numBytes, mask uint32
	// Source embeds m_0, shared by all zero-bit Allocate calls. No valid bits
	// address this root; the zero arena ref denotes its manager-local identity.
	zeroWord uint32
	closed   bool
}

func NewFixedManager(n uint32, config arena.VectorConfig) (*FixedManager, error) {
	// The source (n+31)/32 overflows above this bound; reject that broken size
	// before any root or allocator is constructed. No giant allocation occurs.
	if n > ^uint32(0)-31 {
		return nil, arena.ErrBounds
	}
	root := uint64(reflect.TypeOf(FixedManager{}).Size())
	if e := config.Descriptors.Budget.Reserve(root); e != nil {
		return nil, e
	}
	pool, e := arena.NewWords(config)
	if e != nil {
		config.Descriptors.Budget.Release(root)
		return nil, e
	}
	nw := numWords(n)
	return &FixedManager{words: pool, budget: config.Descriptors.Budget, rootBytes: root, numBits: n, numWords: nw, numBytes: nw * 4, mask: lastMask(n)}, nil
}
func (m *FixedManager) NumBits() uint32  { return m.numBits }
func (m *FixedManager) NumWords() uint32 { return m.numWords }
func (m *FixedManager) NumBytes() uint32 { return m.numBytes }
func (m *FixedManager) Mask() uint32     { return m.mask }
func (m *FixedManager) validate(r FixedRef) error {
	if m.closed {
		return arena.ErrClosed
	}
	if m.numWords == 0 {
		if r != (FixedRef{}) {
			return arena.ErrInvalidSlot
		}
		return nil
	}
	if !m.words.IsLive(r) {
		return arena.ErrInvalidSlot
	}
	return nil
}

// Allocate leaves contents unspecified in source. The pooled Go allocator
// supplies zero words; callers needing guaranteed initialization use Allocate0.
func (m *FixedManager) Allocate() (FixedRef, error) {
	if m.closed {
		return FixedRef{}, arena.ErrClosed
	}
	return m.words.Allocate(m.numWords, m.numWords)
}
func (m *FixedManager) Allocate0() (FixedRef, error) {
	r, e := m.Allocate()
	if e != nil {
		return r, e
	}
	return r, m.Fill0(r)
}
func (m *FixedManager) Allocate1() (FixedRef, error) {
	r, e := m.Allocate()
	if e != nil {
		return r, e
	}
	return r, m.Fill1(r)
}

// Clone translates allocate(existing-vector); Assign translates copy.
func (m *FixedManager) Clone(src FixedRef) (FixedRef, error) {
	if e := m.validate(src); e != nil {
		return FixedRef{}, e
	}
	r, e := m.Allocate()
	if e != nil {
		return r, e
	}
	if e = m.Copy(r, src); e != nil {
		m.Deallocate(r)
		return FixedRef{}, e
	}
	return r, nil
}
func (m *FixedManager) Deallocate(r FixedRef) error {
	if e := m.validate(r); e != nil {
		return e
	}
	return m.words.Free(r)
}
func (m *FixedManager) Reset() {
	if m.closed {
		return
	}
	m.words.Reset()
}
func (m *FixedManager) Close() {
	if m.closed {
		return
	}
	m.words.Close()
	m.budget.Release(m.rootBytes)
	m.rootBytes = 0
	m.closed = true
}
func (m *FixedManager) Stats() arena.VectorStats { return m.words.Stats() }
func (m *FixedManager) Word(r FixedRef, i uint32) (uint32, error) {
	if e := m.validate(r); e != nil {
		return 0, e
	}
	if i >= m.numWords {
		return 0, arena.ErrBounds
	}
	return m.words.Read(r, i)
}

// WriteWord is the internal manager-owned counterpart of source friend access
// to raw m_data words (needed by tbv_manager); unused high bits are retained.
func (m *FixedManager) WriteWord(r FixedRef, i, v uint32) error {
	if e := m.validate(r); e != nil {
		return e
	}
	if i >= m.numWords {
		return arena.ErrBounds
	}
	return m.words.Write(r, i, v)
}
func (m *FixedManager) Get(r FixedRef, i uint32) (bool, error) {
	if e := m.validate(r); e != nil {
		return false, e
	}
	if i >= m.numBits {
		return false, arena.ErrBounds
	}
	w, e := m.words.Read(r, i/32)
	return w&(uint32(1)<<(i%32)) != 0, e
}
func (m *FixedManager) Set(r FixedRef, i uint32, val bool) error {
	if e := m.validate(r); e != nil {
		return e
	}
	if i >= m.numBits {
		return arena.ErrBounds
	}
	w, e := m.words.Read(r, i/32)
	if e != nil {
		return e
	}
	mask := uint32(1) << (i % 32)
	if val {
		w |= mask
	} else {
		w &= ^mask
	}
	return m.words.Write(r, i/32, w)
}
func (m *FixedManager) Unset(r FixedRef, i uint32) error { return m.Set(r, i, false) }
func (m *FixedManager) SetRange(dst, src FixedRef, hi, lo uint32) error {
	if e := m.validate(dst); e != nil {
		return e
	}
	if e := m.validate(src); e != nil {
		return e
	}
	if lo > hi || hi >= m.numBits {
		return arena.ErrBounds
	}
	if lo%32 == 0 {
		n := (hi - lo + 1) / 32
		start := lo / 32
		for i := uint32(0); i < n; i++ {
			w, e := m.words.Read(src, i)
			if e != nil {
				return e
			}
			if e = m.words.Write(dst, start+i, w); e != nil {
				return e
			}
		}
		for i := n * 32; i < hi-lo+1; i++ {
			v, e := m.Get(src, i)
			if e != nil {
				return e
			}
			if e = m.Set(dst, lo+i, v); e != nil {
				return e
			}
		}
		return nil
	}
	for i := uint32(0); i < hi-lo+1; i++ {
		v, e := m.Get(src, i)
		if e != nil {
			return e
		}
		if e = m.Set(dst, lo+i, v); e != nil {
			return e
		}
	}
	return nil
}
func (m *FixedManager) Copy(dst, src FixedRef) error {
	if e := m.validate(dst); e != nil {
		return e
	}
	if e := m.validate(src); e != nil {
		return e
	}
	for i := uint32(0); i < m.numWords; i++ {
		w, e := m.words.Read(src, i)
		if e != nil {
			return e
		}
		if e = m.words.Write(dst, i, w); e != nil {
			return e
		}
	}
	return nil
}
func (m *FixedManager) Assign(dst, src FixedRef) error { return m.Copy(dst, src) }
func (m *FixedManager) Fill0(r FixedRef) error         { return m.fill(r, 0) }
func (m *FixedManager) Fill1(r FixedRef) error         { return m.fill(r, ^uint32(0)) }
func (m *FixedManager) ResetVector(r FixedRef) error   { return m.Fill0(r) }
func (m *FixedManager) fill(r FixedRef, val uint32) error {
	if e := m.validate(r); e != nil {
		return e
	}
	for i := uint32(0); i < m.numWords; i++ {
		if e := m.words.Write(r, i, val); e != nil {
			return e
		}
	}
	return nil
}
func (m *FixedManager) And(dst, src FixedRef) error {
	if e := m.validate(dst); e != nil {
		return e
	}
	if e := m.validate(src); e != nil {
		return e
	}
	for i := uint32(0); i < m.numWords; i++ {
		a, e := m.words.Read(dst, i)
		if e != nil {
			return e
		}
		b, e := m.words.Read(src, i)
		if e != nil {
			return e
		}
		if e = m.words.Write(dst, i, a&b); e != nil {
			return e
		}
	}
	return nil
}
func (m *FixedManager) Or(dst, src FixedRef) error {
	if e := m.validate(dst); e != nil {
		return e
	}
	if e := m.validate(src); e != nil {
		return e
	}
	for i := uint32(0); i < m.numWords; i++ {
		a, e := m.words.Read(dst, i)
		if e != nil {
			return e
		}
		b, e := m.words.Read(src, i)
		if e != nil {
			return e
		}
		if e = m.words.Write(dst, i, a|b); e != nil {
			return e
		}
	}
	return nil
}
func (m *FixedManager) Neg(r FixedRef) error {
	if e := m.validate(r); e != nil {
		return e
	}
	for i := uint32(0); i < m.numWords; i++ {
		w, e := m.words.Read(r, i)
		if e != nil {
			return e
		}
		if e = m.words.Write(r, i, ^w); e != nil {
			return e
		}
	}
	return nil
}
func (m *FixedManager) LastWord(r FixedRef) (uint32, error) {
	if e := m.validate(r); e != nil {
		return 0, e
	}
	if m.numWords == 0 {
		return 0, nil
	}
	w, e := m.words.Read(r, m.numWords-1)
	return w & m.mask, e
}
func (m *FixedManager) Equals(a, b FixedRef) (bool, error) {
	if e := m.validate(a); e != nil {
		return false, e
	}
	if e := m.validate(b); e != nil {
		return false, e
	}
	if a == b || m.numWords == 0 {
		return true, nil
	}
	for i := uint32(0); i < m.numWords-1; i++ {
		w, e := m.words.Read(a, i)
		if e != nil {
			return false, e
		}
		x, e := m.words.Read(b, i)
		if e != nil {
			return false, e
		}
		if w != x {
			return false, nil
		}
	}
	w, e := m.LastWord(a)
	if e != nil {
		return false, e
	}
	x, e := m.LastWord(b)
	return w == x, e
}
func (m *FixedManager) Contains(a, b FixedRef) (bool, error) {
	if e := m.validate(a); e != nil {
		return false, e
	}
	if e := m.validate(b); e != nil {
		return false, e
	}
	if m.numWords == 0 {
		return true, nil
	}
	for i := uint32(0); i < m.numWords-1; i++ {
		w, e := m.words.Read(a, i)
		if e != nil {
			return false, e
		}
		x, e := m.words.Read(b, i)
		if e != nil {
			return false, e
		}
		if w&x != x {
			return false, nil
		}
	}
	w, e := m.LastWord(a)
	if e != nil {
		return false, e
	}
	x, e := m.LastWord(b)
	return w&x == x, e
}
func (m *FixedManager) Hash(r FixedRef) (uint32, error) {
	if e := m.validate(r); e != nil {
		return 0, e
	}
	var err error
	h := wordReader(m.numBits/8, m.numBits, func(i uint32) uint32 {
		w, e := m.words.Read(r, i)
		if e != nil && err == nil {
			err = e
		}
		return w
	})
	return h, err
}
func (m *FixedManager) String(r FixedRef) (string, error) {
	if e := m.validate(r); e != nil {
		return "", e
	}
	var out strings.Builder
	out.Grow(int(m.numBits))
	for i := m.numBits; i > 0; {
		i--
		v, e := m.Get(r, i)
		if e != nil {
			return "", e
		}
		if v {
			out.WriteByte('1')
		} else {
			out.WriteByte('0')
		}
	}
	return out.String(), nil
}
