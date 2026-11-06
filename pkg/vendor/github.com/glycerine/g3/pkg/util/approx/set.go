package approx

// Source: src/util/approx_set.h:43-158,162-233; src/util/approx_set.cpp:22-51.

import (
	"math/bits"
	"strconv"
	"strings"
)

// UintSet is the concrete uint32, 64-bit approximate-set specialization.
// Values with the same residue modulo 64 deliberately collide.
type UintSet struct{ bits uint64 }

func NewUintSet(element uint32) UintSet {
	return UintSet{bits: uint64(1) << (element & 63)}
}

func UintSetFrom(elements []uint32) UintSet {
	var s UintSet
	for _, element := range elements {
		s.Insert(element)
	}
	return s
}

func (s *UintSet) Insert(element uint32) { s.bits |= uint64(1) << (element & 63) }
func (s UintSet) MayContain(element uint32) bool {
	return s.bits&(uint64(1)<<(element&63)) != 0
}
func (s UintSet) MustNotContain(element uint32) bool { return !s.MayContain(element) }
func (s UintSet) Union(other UintSet) UintSet        { return UintSet{bits: s.bits | other.bits} }
func (s UintSet) Intersection(other UintSet) UintSet {
	return UintSet{bits: s.bits & other.bits}
}
func (s *UintSet) UnionWith(other UintSet)          { s.bits |= other.bits }
func (s *UintSet) IntersectWith(other UintSet)      { s.bits &= other.bits }
func (s *UintSet) Subtract(other UintSet)           { s.bits &^= other.bits }
func (s UintSet) Empty() bool                       { return s.bits == 0 }
func (s UintSet) MustNotSubset(other UintSet) bool  { return s.bits&^other.bits != 0 }
func (s UintSet) MustNotSubsume(other UintSet) bool { return s.MustNotSubset(other) }
func (s UintSet) MustNotEqual(other UintSet) bool   { return s.bits != other.bits }
func (s UintSet) MayEqual(other UintSet) bool       { return s.bits == other.bits }
func (s UintSet) Equivalent(other UintSet) bool     { return s.bits == other.bits }
func (s UintSet) ApproxSubset(other UintSet) bool   { return s.bits&^other.bits == 0 }
func (s *UintSet) Reset()                           { s.bits = 0 }
func (s UintSet) EmptyIntersection(other UintSet) bool {
	return s.bits&other.bits == 0
}
func (s UintSet) Size() uint32 { return uint32(bits.OnesCount64(s.bits)) }

// ForEach visits the represented residues in ascending order. Returning false
// stops the traversal.
func (s UintSet) ForEach(yield func(uint32) bool) {
	word := s.bits
	for word != 0 {
		value := uint32(bits.TrailingZeros64(word))
		if !yield(value) {
			return
		}
		word &^= uint64(1) << value
	}
}

func (s UintSet) String() string {
	var out strings.Builder
	out.WriteByte('{')
	first := true
	s.ForEach(func(value uint32) bool {
		if !first {
			out.WriteString(", ")
		}
		first = false
		out.WriteString(strconv.FormatUint(uint64(value), 10))
		return true
	})
	out.WriteByte('}')
	return out.String()
}
