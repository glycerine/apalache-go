package bitset

// Translation of src/util/bit_vector.h and bit_vector.cpp.

import (
	"strings"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// Vector owns 32-bit words in a Go slice and preserves unused bit values.
// Copy with Clone or Assign; Go struct assignment aliases the word storage.
type Vector struct {
	size  uint32
	words []uint32
	trace *diagnostic.TraceTags
}

func numWords(n uint32) uint32       { return uint32((uint64(n) + 31) / 32) }
func New(reserveBits uint32) *Vector { return &Vector{words: make([]uint32, numWords(reserveBits))} }
func FromWords(words []uint32, n uint32) *Vector {
	v := New(n)
	v.size = n
	copy(v.words, words[:numWords(n)])
	return v
}
func (v *Vector) Clone() *Vector {
	w := &Vector{size: v.size, trace: v.trace}
	if v.words != nil {
		w.words = append([]uint32{}, v.words...)
	}
	return w
}
func (v *Vector) Assign(src *Vector) {
	v.size = src.size
	if src.words == nil {
		return
	}
	if len(v.words) < len(src.words) {
		v.words = make([]uint32, len(src.words))
	}
	copy(v.words, src.words)
}
func (v *Vector) Reset() { clear(v.words); v.size = 0 }
func (v *Vector) Swap(other *Vector) {
	v.size, other.size = other.size, v.size
	v.words, other.words = other.words, v.words
}
func (v *Vector) Fill0()               { clear(v.words) }
func (v *Vector) Size() uint32         { return v.size }
func (v *Vector) Empty() bool          { return v.size == 0 }
func (v *Vector) NumWords() uint32     { return numWords(v.size) }
func (v *Vector) Word(i uint32) uint32 { return v.words[i] }
func (v *Vector) check(i uint32) {
	if i >= v.size {
		panic("bitset: bit index out of range")
	}
}
func (v *Vector) Get(i uint32) bool { v.check(i); return v.words[i/32]&(uint32(1)<<(i%32)) != 0 }
func (v *Vector) Set(i uint32, val bool) {
	v.check(i)
	mask := uint32(1) << (i % 32)
	if val {
		v.words[i/32] |= mask
	} else {
		v.words[i/32] &= ^mask
	}
}
func (v *Vector) Unset(i uint32) { v.Set(i, false) }
func (v *Vector) Push(val bool) {
	i := v.size
	v.size++
	if v.size == 0 {
		v.size = i
		panic("bitset: size overflow")
	}
	if v.NumWords() > uint32(len(v.words)) {
		v.words = append(v.words, 0)
	}
	v.Set(i, val)
}
func (v *Vector) Pop() {
	if v.Empty() {
		panic("bitset: pop empty vector")
	}
	v.size--
}
func (v *Vector) Back() bool {
	if v.Empty() {
		panic("bitset: back empty vector")
	}
	return v.Get(v.size - 1)
}
func (v *Vector) Shrink(n uint32) {
	if n > v.size {
		panic("bitset: shrink grows vector")
	}
	v.size = n
}
func (v *Vector) Resize(n uint32, val bool) {
	if n <= v.size {
		v.size = n
		return
	}
	nw := numWords(n)
	v.traceResizeStart(n, nw)
	if nw > uint32(len(v.words)) {
		v.words = append(v.words, make([]uint32, int(nw)-len(v.words))...)
	}
	bw := v.size / 32
	mask := (uint32(1) << (v.size % 32)) - 1
	fill := uint32(0)
	if val {
		v.words[bw] |= ^mask
		fill = ^uint32(0)
	} else {
		v.words[bw] &= mask
	}
	v.traceResizeFields(bw, nw, v.size%32, mask, val)
	for i := bw + 1; i < nw; i++ {
		v.words[i] = fill
	}
	v.size = n
}
func (v *Vector) Reserve(n uint32, val bool) {
	if n > v.size {
		v.Resize(n, val)
	}
}

// ShiftRight increases bit indices by k and inserts k zero low bits, as in C++.
func (v *Vector) ShiftRight(k uint32) error {
	if k == 0 {
		return nil
	}
	n := v.size + k
	if n < v.size {
		panic("bitset: size overflow")
	}
	old := v.NumWords()
	v.Resize(n, false)
	nw := v.NumWords()
	bits, words := k%32, k/32
	if words > 0 {
		j, i := old, old+words
		for j > 0 {
			j--
			i--
			v.words[i] = v.words[j]
		}
		for i > 0 {
			i--
			v.words[i] = 0
		}
	}
	if bits > 0 {
		// DEBUG_CODE's loop evaluates no SASSERT when word_shift is zero.
		if diagnostic.DebugBuild && words > 0 {
			d := diagnostic.BorrowProcessDebugContext()
			enabled, e := diagnostic.SourceAssertionsEnabled(d)
			if e != nil {
				return e
			}
			if enabled {
				for i := uint32(0); i < words; i++ {
					if v.words[i] != 0 {
						if e := diagnostic.DebugAssertionFailure(d, "src/util/bit_vector.cpp", 106, "m_data[i] == 0"); e != nil {
							return e
						}
					}
				}
			}
		}
		prev := uint32(0)
		for i := words; i < nw; i++ {
			next := v.words[i] >> (32 - bits)
			v.words[i] = (v.words[i] << bits) | prev
			prev = next
		}
	}
	return nil
}
func lastMask(n uint32) uint32 {
	m := (uint32(1) << (n % 32)) - 1
	if m == 0 {
		return ^uint32(0)
	}
	return m
}
func (v *Vector) Equal(other *Vector) bool {
	if v.size != other.size {
		return false
	}
	n := v.NumWords()
	if n == 0 {
		return true
	}
	for i := uint32(0); i < n-1; i++ {
		if v.words[i] != other.words[i] {
			return false
		}
	}
	mask := lastMask(v.size)
	return v.words[n-1]&mask == other.words[n-1]&mask
}
func (v *Vector) Or(src *Vector) error {
	if v.size < src.size {
		v.Resize(src.size, false)
	}
	n := src.NumWords()
	if diagnostic.DebugBuild {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return e
		}
		if enabled && n > v.NumWords() {
			if e := diagnostic.DebugAssertionFailure(d, "src/util/bit_vector.cpp", 141, "n2 <= num_words()"); e != nil {
				return e
			}
		}
	}
	if n == 0 {
		return nil
	}
	for i := uint32(0); i < n-1; i++ {
		v.words[i] |= src.words[i]
	}
	v.words[n-1] |= src.words[n-1] & lastMask(src.size)
	return nil
}
func (v *Vector) And(src *Vector) error {
	n1, n2 := v.NumWords(), src.NumWords()
	if n1 == 0 {
		return nil
	}
	if n2 > n1 {
		for i := uint32(0); i < n1; i++ {
			v.words[i] &= src.words[i]
		}
		return nil
	}
	if diagnostic.DebugBuild {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return e
		}
		if enabled && n2 > n1 {
			if e := diagnostic.DebugAssertionFailure(d, "src/util/bit_vector.cpp", 168, "n2 <= n1"); e != nil {
				return e
			}
		}
	}
	if n2 > 0 {
		for i := uint32(0); i < n2-1; i++ {
			v.words[i] &= src.words[i]
		}
		v.words[n2-1] &= src.words[n2-1] & lastMask(src.size)
	}
	for i := n2; i < n1; i++ {
		v.words[i] = 0
	}
	return nil
}
func (v *Vector) Neg() {
	for i := uint32(0); i < v.NumWords(); i++ {
		v.words[i] = ^v.words[i]
	}
}

// Contains preserves source receiver-length semantics. Other must have at least
// NumWords receiver words in its capacity; excess other bits are not considered.
func (v *Vector) Contains(other *Vector) bool {
	n := v.NumWords()
	if n == 0 {
		return true
	}
	for i := uint32(0); i < n-1; i++ {
		if v.words[i]&other.words[i] != other.words[i] {
			return false
		}
	}
	d := other.words[n-1] & lastMask(v.size)
	return v.words[n-1]&d == d
}
func (v *Vector) String() string {
	var b strings.Builder
	b.Grow(int(v.size))
	for i := v.size; i > 0; {
		i--
		if v.Get(i) {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
	}
	return b.String()
}

// Hash preserves get_hash's floor(size/8) byte length, including omission of
// incomplete final bytes. Word bytes follow the source little-endian baseline.
func (v *Vector) Hash() uint32 { return wordBytes(v.words, v.size/8, 0) }
