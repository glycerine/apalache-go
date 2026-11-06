package numeral

// Source: src/util/mpn.cpp to_string and src/util/mpz.cpp/src/util/mpq.cpp display.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

var ErrShortBuffer = errors.New("mpnum: decimal output buffer too short")

// reserveText grows the reusable Go output assembly while preserving prefixes
// already written, including the numerator before a rational denominator.
func (m *NaturalManager) reserveText(needed uint32) error {
	if uint64(needed) > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	if int(needed) > len(m.text) {
		m.text = append(m.text, make([]byte, int(needed)-len(m.text))...)
	}
	return nil
}
func (m *NaturalManager) textRead(i uint32) byte     { return m.text[i] }
func (m *NaturalManager) textWrite(i uint32, v byte) { m.text[i] = v }
func (m *NaturalManager) reverseText(start, end uint32) {
	for start < end {
		end--
		if start >= end {
			break
		}
		a, b := m.textRead(start), m.textRead(end)
		m.textWrite(start, b)
		m.textWrite(end, a)
		start++
	}
}
func checkedTextEnd(offset uint32, n uint64) (uint32, error) {
	if n > math.MaxUint32-uint64(offset) {
		return 0, arena.ErrBounds
	}
	return offset + uint32(n), nil
}

// prepareNatural follows source normalize / div_1 / unnormalize by ten. The
// three source local limb buffers have fresh source lifetimes in the embedded
// charged owner. Text remains an explicit output assembly adapter.
func (m *NaturalManager) prepareNatural(a limbView, offset uint32) (uint32, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	end, e := checkedTextEnd(offset, 11*uint64(a.size))
	if e != nil {
		return 0, e
	}
	if end == offset {
		end, e = checkedTextEnd(offset, 1)
		if e != nil {
			return 0, e
		}
	}
	if e = m.reserveText(end); e != nil {
		return 0, e
	}
	if e = m.sourceToCString(a, sourceDecimalDestination{assembly: m, offset: offset}, uint64(end-offset)); e != nil {
		return 0, e
	}
	for j := offset; j < end; j++ {
		if m.textRead(j) == 0 {
			return j, nil
		}
	}
	return 0, arena.ErrBounds
}
func (m *NaturalManager) publishText(dst []byte, n uint32) (int, error) {
	if uint64(n) > uint64(^uint(0)>>1) {
		return 0, arena.ErrBounds
	}
	if uint64(n) > uint64(len(dst)) {
		return int(n), ErrShortBuffer
	}
	copy(dst, m.text[:n])
	return int(n), nil
}

// finalString copies the prepared bytes into the caller-owned Go string.
func (m *NaturalManager) finalString(n uint32) (string, error) {
	if uint64(n) > uint64(^uint(0)>>1) {
		return "", arena.ErrBounds
	}
	return string(m.text[:n]), nil
}

// FormatInto excludes the C terminator. It writes exactly n bytes, leaving dst
// unchanged on a short buffer or predictable storage failure.
func (m *NaturalManager) FormatInto(a []uint32, dst []byte) (int, error) {
	if uint64(len(a)) > math.MaxUint32 {
		return 0, arena.ErrBounds
	}
	n, e := m.prepareNatural(limbView{external: a, size: uint32(len(a))}, 0)
	if e != nil {
		return 0, e
	}
	return m.publishText(dst, n)
}
func (m *NaturalManager) String(a []uint32) (string, error) {
	if uint64(len(a)) > math.MaxUint32 {
		return "", arena.ErrBounds
	}
	n, e := m.prepareNatural(limbView{external: a, size: uint32(len(a))}, 0)
	if e != nil {
		return "", e
	}
	return m.finalString(n)
}
func (m *IntegerManager) prepareInteger(a *Int, offset uint32) (uint32, error) {
	if e := m.check(a); e != nil {
		return 0, e
	}
	if a.IsSmall() {
		value := int64(a.Val)
		end, e := checkedTextEnd(offset, 12)
		if e != nil {
			return 0, e
		}
		if e = m.Natural.reserveText(end); e != nil {
			return 0, e
		}
		if value < 0 {
			m.Natural.textWrite(offset, '-')
			offset++
			value = -value
		}
		j := offset
		for {
			m.Natural.textWrite(j, '0'+byte(value%10))
			j++
			value /= 10
			if value == 0 {
				break
			}
		}
		m.Natural.reverseText(offset, j)
		return j, nil
	}
	if a.Val < 0 {
		end, e := checkedTextEnd(offset, 1)
		if e != nil {
			return 0, e
		}
		if e = m.Natural.reserveText(end); e != nil {
			return 0, e
		}
		m.Natural.textWrite(offset, '-')
		offset = end
	}
	c := m.Cell(a.Cell)
	size, e := c.Backing.ReadWord(m.Allocator, 0)
	if e != nil {
		return 0, e
	}
	// Source unsigned multiplication precedes sbuffer construction. Its
	// physical inline capacity remains independent of the wrapped POS.
	if e = m.Natural.beginIntegerCharacters(uint32(11 * size)); e != nil {
		return 0, e
	}
	natural := m.Natural
	e = natural.sourceToCString(limbView{allocator: m.Allocator, backing: c.Backing, size: size}, sourceDecimalDestination{character: natural.CharFormatting.Buffer}, uint64(len(natural.CharFormatting.Buffer)))
	var n uint32
	if e == nil {
		n, e = natural.appendIntegerCString(offset)
	}
	if e != nil {
		natural.CharFormatting.originalCause = e
	}
	// An interrupted inner destructor retains its cursor. The outer completed
	// char object cannot be destroyed ahead of that actual source prefix.
	if natural.Formatting.CompletedField != 0 {
		return 0, e
	}
	cleanup := natural.destroyIntegerCharacters()
	if e != nil {
		return 0, e
	}
	if cleanup != nil {
		return 0, cleanup
	}
	return n, nil
}
func (m *IntegerManager) FormatInto(a *Int, dst []byte) (int, error) {
	n, e := m.prepareInteger(a, 0)
	if e != nil {
		return 0, e
	}
	return m.Natural.publishText(dst, n)
}
func (m *IntegerManager) String(a *Int) (string, error) {
	n, e := m.prepareInteger(a, 0)
	if e != nil {
		return "", e
	}
	return m.Natural.finalString(n)
}
func (m *RationalManager) prepareRational(a *Rational) (uint32, error) {
	if e := m.check(a); e != nil {
		return 0, e
	}
	n, e := m.Z.prepareInteger(&a.Num, 0)
	if e != nil {
		return 0, e
	}
	if isIntegerRational(a) {
		return n, nil
	}
	end, e := checkedTextEnd(n, 1)
	if e != nil {
		return 0, e
	}
	if e = m.Z.Natural.reserveText(end); e != nil {
		return 0, e
	}
	m.Z.Natural.textWrite(n, '/')
	return m.Z.prepareInteger(&a.Den, end)
}
func (m *RationalManager) FormatInto(a *Rational, dst []byte) (int, error) {
	n, e := m.prepareRational(a)
	if e != nil {
		return 0, e
	}
	return m.Z.Natural.publishText(dst, n)
}
func (m *RationalManager) String(a *Rational) (string, error) {
	n, e := m.prepareRational(a)
	if e != nil {
		return "", e
	}
	return m.Z.Natural.finalString(n)
}
