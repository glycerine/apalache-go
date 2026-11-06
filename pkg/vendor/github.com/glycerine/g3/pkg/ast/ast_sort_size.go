package ast

// Translation of src/ast/ast.h::sort_size and ast.cpp stream insertion.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

var ErrSortSizeNotFinite = errors.New("g3: sort size is not finite")

// SortSize is a pointer-free value, freely copyable with ordinary assignment.
// Private kind encoding0=infinite,1=finite,2=very-big makes the Go zero value
// match the source default constructor. Source enum values are not exposed.
// Nonfinite size is canonical0; source default m_size was unspecified.
type SortSize struct {
	kind uint32
	size uint64
}

func NewSortSize(size uint64) SortSize    { return FiniteSortSize(size) }
func FiniteSortSize(size uint64) SortSize { return SortSize{kind: 1, size: size} }
func VeryBigSortSize() SortSize           { return SortSize{kind: 2} }
func InfiniteSortSize() SortSize          { return SortSize{} }

// SortSizeFromRational preserves the actual source IsUint64 -> Uint64 route.
// Fractions/negative/too-large values become very-big, not a domain error.
// The service and its token are borrowed for these calls only. The caller must
// not concurrently mutate/delete the borrowed operand between the two actions.
func SortSizeFromRational(m *unumeral.RationalService, r *unumeral.OwnedRational) (SortSize, error) {
	if m == nil {
		return SortSize{}, unumeral.ErrRationalServiceOwner
	}
	valid, e := m.IsUint64(r)
	if e != nil {
		return SortSize{}, e
	}
	if !valid {
		return VeryBigSortSize(), nil
	}
	n, e := m.Uint64(r)
	if e != nil {
		return SortSize{}, e
	}
	return FiniteSortSize(n), nil
}
func (s SortSize) IsInfinite() bool            { return s.kind == 0 }
func (s SortSize) IsVeryBig() bool             { return s.kind == 2 }
func (s SortSize) IsFinite() bool              { return s.kind == 1 }
func SortSizeIsVeryBigBase2(power uint32) bool { return power >= 64 }

// Size guards the source SASSERT precondition in every Go profile instead of
// reading an unspecified/irrelevant nonfinite field in release builds.
func (s SortSize) Size() (uint64, error) {
	if !s.IsFinite() {
		return 0, ErrSortSizeNotFinite
	}
	return s.size, nil
}

// String uses stack digits and one final caller-owned string, without a host
// formatting intermediate. Nonfinite constant text needs no allocation.
func (s SortSize) String() string {
	if s.IsInfinite() {
		return "infinite"
	}
	if s.IsVeryBig() {
		return "very-big"
	}
	var digits [20]byte
	at := len(digits)
	n := s.size
	for {
		at--
		digits[at] = '0' + byte(n%10)
		n /= 10
		if n == 0 {
			break
		}
	}
	return string(digits[at:])
}
