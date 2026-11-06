package bitset

// Concrete unsigned approx_set from src/util/approx_set.h and approx_set.cpp.

import (
	"math/bits"
	"strconv"
	"strings"
)

// ApproxSet is the source 64-bit approximation of unsigned labels. Values
// differing by a multiple of 64 intentionally collide.
type ApproxSet uint64

func NewApproxSet(values ...uint32) ApproxSet {
	var s ApproxSet
	for _, v := range values {
		s.Insert(v)
	}
	return s
}

func (s *ApproxSet) Insert(v uint32)                   { *s |= 1 << (v & 63) }
func (s ApproxSet) MayContain(v uint32) bool           { return s&(1<<(v&63)) != 0 }
func (s ApproxSet) MustNotContain(v uint32) bool       { return !s.MayContain(v) }
func (s ApproxSet) Union(t ApproxSet) ApproxSet        { return s | t }
func (s ApproxSet) Intersect(t ApproxSet) ApproxSet    { return s & t }
func (s *ApproxSet) UnionAssign(t ApproxSet)           { *s |= t }
func (s *ApproxSet) IntersectAssign(t ApproxSet)       { *s &= t }
func (s *ApproxSet) SubtractAssign(t ApproxSet)        { *s &^= t }
func (s ApproxSet) Empty() bool                        { return s == 0 }
func (s ApproxSet) MustNotSubset(t ApproxSet) bool     { return s&^t != 0 }
func (s ApproxSet) MustNotSubsume(t ApproxSet) bool    { return s.MustNotSubset(t) }
func (s ApproxSet) MustNotEqual(t ApproxSet) bool      { return s != t }
func (s ApproxSet) MayEqual(t ApproxSet) bool          { return s == t }
func (s ApproxSet) Equivalent(t ApproxSet) bool        { return s == t }
func (s ApproxSet) ApproxSubset(t ApproxSet) bool      { return s|t == t }
func (s *ApproxSet) Reset()                            { *s = 0 }
func (s ApproxSet) EmptyIntersection(t ApproxSet) bool { return s&t == 0 }

// ForEach visits occupied residue positions in source iterator order.
// Stopping early is a Go caller convenience; the source iterator is a value.
func (s ApproxSet) ForEach(visit func(uint32) bool) {
	for remaining := uint64(s); remaining != 0; remaining &= remaining - 1 {
		if !visit(uint32(bits.TrailingZeros64(remaining))) {
			return
		}
	}
}

func (s ApproxSet) Size() uint32 { return uint32(bits.OnesCount64(uint64(s))) }

// String matches approx_set::display's ascending, comma-space format.
func (s ApproxSet) String() string {
	var b strings.Builder
	b.WriteByte('{')
	first := true
	s.ForEach(func(v uint32) bool {
		if !first {
			b.WriteString(", ")
		}
		first = false
		b.WriteString(strconv.FormatUint(uint64(v), 10))
		return true
	})
	b.WriteByte('}')
	return b.String()
}
