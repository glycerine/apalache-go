package numeral

// Scalar inspection from src/util/mpz.h/.cpp, internal 32-bit digit baseline.

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uhash "github.com/glycerine/g3/pkg/util/hash"
)

func (m *IntegerManager) Precise() bool { return true }
func (m *IntegerManager) Field() bool   { return false }

// IsUint32 retains source is_uint's strict comparison: UINT_MAX is excluded.
func (m *IntegerManager) IsUint32(a *Int) (bool, error) {
	ok, e := m.IsUint64(a)
	if e != nil || !ok {
		return false, e
	}
	v, e := m.Uint64(a)
	return v < uint64(^uint32(0)), e
}
func (m *IntegerManager) Uint32(a *Int) (uint32, error) {
	if modularSourceDebug {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return 0, e
		}
		if enabled {
			ok, e := m.IsUint32(a)
			if e != nil {
				return 0, e
			}
			if !ok {
				if e = diagnostic.DebugAssertionFailure(d, "src/util/mpz.h", 738, "is_uint(a)"); e != nil {
					return 0, e
				}
			}
		}
	}
	v, e := m.Uint64(a)
	return uint32(v), e
}

// IsInt32 retains source is_int's exclusion of both INT_MIN and INT_MAX.
func (m *IntegerManager) IsInt32(a *Int) (bool, error) {
	ok, e := m.IsInt64(a)
	if e != nil || !ok {
		return false, e
	}
	v, e := m.Int64(a)
	return v > -2147483648 && v < 2147483647, e
}
func (m *IntegerManager) Int32(a *Int) (int32, error) {
	if modularSourceDebug {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return 0, e
		}
		if enabled {
			ok, e := m.IsInt32(a)
			if e != nil {
				return 0, e
			}
			if !ok {
				if e = diagnostic.DebugAssertionFailure(d, "src/util/mpz.h", 742, "is_int(a)"); e != nil {
					return 0, e
				}
			}
		}
	}
	v, e := m.Int64(a)
	return int32(v), e
}

func (m *IntegerManager) IsEven(a *Int) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	if a.IsSmall() {
		return a.Val&1 == 0, nil
	}
	return m.Digit(a.Cell, 0)&1 == 0, nil
}
func (m *IntegerManager) IsOdd(a *Int) (bool, error) {
	even, e := m.IsEven(a)
	if e != nil {
		return false, e
	}
	return !even, nil
}

// SizeInfo is the source sorting weight, not an allocation byte count.
func (m *IntegerManager) SizeInfo(a *Int) (uint32, error) {
	if e := m.check(a); e != nil {
		return 0, e
	}
	if a.IsSmall() {
		return 1, nil
	}
	return m.Cell(a.Cell).Size + 1, nil
}

// Float64 deliberately retains the source UINT_MAX floating radix and every
// multiply/add, including overflow and Inf*0 propagation through interior zeros.
func (m *IntegerManager) Float64(a *Int) (float64, error) {
	if e := m.check(a); e != nil {
		return 0, e
	}
	if a.IsSmall() {
		return float64(a.Val), nil
	}
	r, d := 0.0, 1.0
	for i, n := uint32(0), m.Cell(a.Cell).Size; i < n; i++ {
		product := float64(d * float64(m.Digit(a.Cell, i)))
		r += product
		d *= float64(^uint32(0))
	}
	if a.Val < 0 {
		r = -r
	}
	return r, nil
}

// Hash ignores the sign of big magnitudes, as the selected source does. The
// byte-length expression retains source unsigned narrowing/wrap; bytes are read
// from indexed little-endian storage without exposing or allocating a slice.
func (m *IntegerManager) Hash(a *Int) (uint32, error) {
	if e := m.check(a); e != nil {
		return 0, e
	}
	if a.IsSmall() {
		return uint32(a.Val), nil
	}
	size := m.Cell(a.Cell).Size
	if size == 1 {
		return m.Digit(a.Cell, 0), nil
	}
	return uhash.WordReader(size*4, 17, func(i uint32) uint32 { return m.Digit(a.Cell, i) }), nil
}

// PowerOfTwoMultiple follows the source repeated divisibility checks. Zero has
// result zero. The accumulated count uses the source unsigned wrap semantics.
func (m *IntegerManager) PowerOfTwoMultiple(a *Int) (uint32, error) {
	if e := m.check(a); e != nil {
		return 0, e
	}
	var r uint32
	var v int64
	if a.IsSmall() {
		v = int64(a.Val)
		if v == 0 {
			return 0, nil
		}
	} else {
		for i, n := uint32(0), m.Cell(a.Cell).Size; i < n; i++ {
			v = int64(m.Digit(a.Cell, i))
			if v != 0 {
				break
			}
			r += 32
		}
		if v == 0 {
			return r, nil
		}
	}
	for _, k := range [...]uint32{16, 8, 4, 2, 1} {
		divisor := int64(1) << k
		if v%divisor == 0 {
			r += k
			v /= divisor
		}
	}
	return r, nil
}

// Decompose resets the logical caller buffer and appends all live magnitude
// digits. Its caller-owned output is the only allocation boundary; a sufficiently
// capacious buffer allocates nothing. Inline zero produces one zero digit.
func (m *IntegerManager) Decompose(a *Int, dst []uint32) (bool, []uint32, error) {
	if e := m.check(a); e != nil {
		return false, dst, e
	}
	dst = dst[:0]
	if a.IsSmall() {
		v := int64(a.Val)
		if v < 0 {
			v = -v
		}
		return a.Val < 0, append(dst, uint32(v)), nil
	}
	for i, n := uint32(0), m.Cell(a.Cell).Size; i < n; i++ {
		dst = append(dst, m.Digit(a.Cell, i))
	}
	return a.Val < 0, dst, nil
}
