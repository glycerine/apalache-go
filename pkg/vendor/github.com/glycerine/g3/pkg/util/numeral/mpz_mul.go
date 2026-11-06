package numeral

// Source: src/util/mpz.h::mul, src/util/mpz.cpp::big_mul,
// src/util/mpn.cpp::mul

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

// Mul translates the source small-int path and big multiplication. Exact
// destination/input aliasing is supported through the manager's result scratch.
func (m *IntegerManager) Mul(a, b, c *Int) error {
	return m.mulWithAllocationContext(a, b, c, integerAllocationInvocationContext{})
}
func (m *IntegerManager) mulWithAllocationContext(a, b, c *Int, context integerAllocationInvocationContext) (err error) {
	for _, n := range []*Int{a, b, c} {
		if e := m.check(n); e != nil {
			return e
		}
	}
	m.traceMPZBinary(a, b, c, "*", true)
	defer func() {
		if err == nil {
			m.traceMPZBinary(a, b, c, "*", false)
		}
	}()
	if a.IsSmall() && b.IsSmall() {
		return m.setInt64WithAllocationContext(c, int64(a.Val)*int64(b.Val), context)
	}
	sa, ca, e := m.signCell(a, 0)
	if e != nil {
		return e
	}
	sb, cb, e := m.signCell(b, 1)
	if e != nil {
		return e
	}
	size := uint64(m.Cell(ca).Size) + uint64(m.Cell(cb).Size)
	if size > math.MaxUint32 {
		return arena.ErrBounds
	}
	if e := m.ensureTmpWithAllocationContext(0, uint32(size), context); e != nil {
		return e
	}
	if e := m.prepareTmpSpan(0, uint32(size)); e != nil {
		return e
	}
	m.Natural.traceMPNBinaryView(
		limbView{allocator: m.Allocator, backing: m.Cell(ca).Backing, size: m.Cell(ca).Size},
		limbView{allocator: m.Allocator, backing: m.Cell(cb).Backing, size: m.Cell(cb).Size}, "*")
	m.multiplyCells(ca, cb, m.Tmp[0])
	m.Natural.traceMPNUnaryView(limbView{allocator: m.Allocator, backing: m.Cell(m.Tmp[0]).Backing, size: uint32(size)}, 427, true)
	sign := int32(1)
	if sa != sb {
		sign = -1
	}
	return m.publishTmpAtWithAllocationContext(0, c, sign, uint32(size), context)
}

// multiplyCells is indexed Knuth Algorithm M. Destination must not alias an
// input; public Mul stages it in source-equivalent owner scratch.
func (m *IntegerManager) multiplyCells(a, b, c NumericCellRef) {
	na, nb := m.Cell(a).Size, m.Cell(b).Size
	for i := uint32(0); i < na; i++ {
		m.writeDigit(c, i, 0)
	}
	for j := uint32(0); j < nb; j++ {
		v := m.Digit(b, j)
		if v == 0 {
			m.writeDigit(c, j+na, 0)
			continue
		}
		var k uint32
		for i := uint32(0); i < na; i++ {
			t := uint64(m.Digit(a, i))*uint64(v) + uint64(m.Digit(c, i+j)) + uint64(k)
			m.writeDigit(c, i+j, uint32(t))
			k = uint32(t >> 32)
		}
		m.writeDigit(c, j+na, k)
	}
}
