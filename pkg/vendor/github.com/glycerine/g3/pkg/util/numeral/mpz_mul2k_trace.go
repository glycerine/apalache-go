package numeral

// Source: src/util/mpz.cpp:1703,1709-1711,1747.

import (
	"io"
	"strconv"
)

func (m *IntegerManager) traceMul2kStart(a *Int, k uint32) {
	_ = m.sourceTraceTags().EmitConditional("mpz_mul2k", "mul2k", "src/util/mpz.cpp", 1703,
		func() bool { return true }, func(w io.Writer) error {
			s, err := m.String(a)
			if err != nil {
				return err
			}
			return writeNumeralDisplayPart(w, "mul2k\na: "+s+"\nk: "+strconv.FormatUint(uint64(k), 10)+"\n")
		})
}

func (m *IntegerManager) traceMul2kCapacity(a *Int, wordShift, bitShift, oldSize, newSize uint32) {
	_ = m.sourceTraceTags().EmitConditional("mpz_mul2k", "mul2k", "src/util/mpz.cpp", 1709,
		func() bool { return true }, func(w io.Writer) error {
			if err := writeNumeralDisplayPart(w, "word_shift: "+strconv.FormatUint(uint64(wordShift), 10)+
				"\nbit_shift: "+strconv.FormatUint(uint64(bitShift), 10)+
				"\nold_sz: "+strconv.FormatUint(uint64(oldSize), 10)+
				"\nnew_sz: "+strconv.FormatUint(uint64(newSize), 10)+
				"\na after ensure capacity:\n"); err != nil {
				return err
			}
			s, err := m.String(a)
			if err != nil {
				return err
			}
			return writeNumeralDisplayPart(w, s+"\n")
		})
}

func (m *IntegerManager) traceMul2kResult(a *Int) {
	_ = m.sourceTraceTags().EmitConditional("mpz_mul2k", "mul2k", "src/util/mpz.cpp", 1747,
		func() bool { return true }, func(w io.Writer) error {
			if err := writeNumeralDisplayPart(w, "mul2k result:\n"); err != nil {
				return err
			}
			s, err := m.String(a)
			if err != nil {
				return err
			}
			return writeNumeralDisplayPart(w, s+"\n")
		})
}
