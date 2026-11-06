package numeral

// Source: src/util/mpz.cpp:1171,1181,1651.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *IntegerManager) sourceTraceTags() *diagnostic.TraceTags {
	if m.trace != nil {
		return m.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (m *IntegerManager) traceBitwiseNotChunk(width uint32, v, n uint64) {
	_ = m.sourceTraceTags().EmitConditional("bitwise_not", "bitwise_not", "src/util/mpz.cpp", 1171,
		func() bool { return true }, func(w io.Writer) error {
			return writeNumeralDisplayPart(w, "sz: "+strconv.FormatUint(uint64(width), 10)+
				", v: "+strconv.FormatUint(v, 10)+", n: "+strconv.FormatUint(n, 10)+"\n")
		})
}

func (m *IntegerManager) traceBitwiseNotFinal(width uint32, a, out *Int) {
	_ = m.sourceTraceTags().EmitConditional("bitwise_not", "bitwise_not", "src/util/mpz.cpp", 1181,
		func() bool { return true }, func(w io.Writer) error {
			if err := writeNumeralDisplayPart(w, "sz: "+strconv.FormatUint(uint64(width), 10)+" a: "); err != nil {
				return err
			}
			sa, err := m.String(a)
			if err != nil {
				return err
			}
			if err := writeNumeralDisplayPart(w, sa+" c: "); err != nil {
				return err
			}
			sc, err := m.String(out)
			if err != nil {
				return err
			}
			return writeNumeralDisplayPart(w, sc+"\n")
		})
}

func (m *IntegerManager) traceMachineDiv2k(bitShift, compShift, newSize, size uint32) {
	_ = m.sourceTraceTags().EmitConditional("mpz_2k", "machine_div2k", "src/util/mpz.cpp", 1651,
		func() bool { return true }, func(w io.Writer) error {
			return writeNumeralDisplayPart(w, "bit_shift: "+strconv.FormatUint(uint64(bitShift), 10)+
				", comp_shift: "+strconv.FormatUint(uint64(compShift), 10)+
				", new_sz: "+strconv.FormatUint(uint64(newSize), 10)+
				", sz: "+strconv.FormatUint(uint64(size), 10)+"\n")
		})
}
