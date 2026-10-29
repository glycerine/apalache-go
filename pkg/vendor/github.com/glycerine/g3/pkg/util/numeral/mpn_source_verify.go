package numeral

// Source: src/util/mpn.cpp:206-217 and actual Algorithm M/A bodies.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// sourceVerifyDivision is the source Z3DEBUG lexical temp, after both outputs.
// A concrete Go slice holds q*b+r. It is cleared on entry and truncated on
// every return, including an assertion failure; no allocator receipt is needed.

func (m *NaturalManager) sourceVerifyDivision(a, b, q, r limbView) error {

	if !modularSourceDebug {
		return nil
	}
	if a.size == ^uint32(0) {
		return arena.ErrBounds
	}
	size := a.size + 1
	if uint64(cap(m.verificationScratch)) < uint64(size) {
		m.verificationScratch = make([]uint32, size)
	} else {
		m.verificationScratch = m.verificationScratch[:size]
		clear(m.verificationScratch)
	}
	defer func() { m.verificationScratch = m.verificationScratch[:0] }()
	out := limbView{external: m.verificationScratch, size: size}
	// mpn_manager::mul, Knuth M: initialize lnga cells, then every column.
	m.traceMPNBinaryView(q, b, "*")
	for i := uint32(0); i < q.size; i++ {
		out.write(i, 0)
	}
	for j := uint32(0); j < b.size; j++ {
		digit := b.read(j)
		if digit == 0 {
			out.write(j+q.size, 0)
			continue
		}
		var carry uint32
		for i := uint32(0); i < q.size; i++ {
			value := uint64(q.read(i))*uint64(digit) + uint64(out.read(i+j)) + uint64(carry)
			out.write(i+j, uint32(value))
			carry = uint32(value >> 32)
		}
		out.write(j+q.size, carry)
	}
	m.traceMPNUnaryView(out, 427, true)
	// Source aliasing add reads temp at each position before writing that cell.
	m.traceMPNBinaryView(limbView{external: m.verificationScratch, size: a.size}, r, "+")
	var carry uint32
	for i := uint32(0); i < a.size; i++ {
		u := out.read(i)
		var v uint32
		if i < r.size {
			v = r.read(i)
		}
		sum := u + v
		c1 := sum < u
		value := sum + carry
		c2 := value < sum
		out.write(i, value)
		carry = 0
		if c1 || c2 {
			carry = 1
		}
	}
	out.write(a.size, carry)
	// The source add also walks the normalized size even though the caller does
	// not consume it. Preserve these reads rather than replacing the algorithm.
	realSize := a.size + 1
	for realSize > 1 && out.read(realSize-1) == 0 {
		realSize--
	}
	m.traceMPNUnaryView(limbView{external: m.verificationScratch, size: realSize}, 427, true)
	ok := true
	for i := uint32(0); i < a.size && ok; i++ {
		if out.read(i) != a.read(i) {
			ok = false
		}
	}
	if out.read(a.size) != 0 {
		ok = false
	}
	m.traceDivisionVerification(ok, out)
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil {
		return e
	}
	if enabled && !ok {
		return diagnostic.DebugAssertionFailure(d, "src/util/mpn.cpp", 216, "ok")
	}
	return nil
}
