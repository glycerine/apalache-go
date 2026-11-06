package numeral

// Source: src/util/mpz.cpp:1021,1034,1037,1039,1987.

import (
	"io"
	"strconv"
)

func (m *IntegerManager) traceBitwiseOrStart(a, b *Int) {
	_ = m.sourceTraceTags().EmitConditional("mpz", "bitwise_or", "src/util/mpz.cpp", 1021,
		func() bool { return true }, func(w io.Writer) error {
			bit := func(v *Int) string {
				if v.IsSmall() {
					return "1"
				}
				return "0"
			}
			return writeNumeralDisplayPart(w, "is_small(a): "+bit(a)+", is_small(b): "+bit(b)+"\n")
		})
}

func (m *IntegerManager) traceBitwiseOrPair(line int, labelA, labelB string, a, b *Int) {
	_ = m.sourceTraceTags().EmitConditional("mpz", "bitwise_or", "src/util/mpz.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			if err := writeNumeralDisplayPart(w, labelA); err != nil {
				return err
			}
			sa, err := m.String(a)
			if err != nil {
				return err
			}
			if err := writeNumeralDisplayPart(w, sa+", "+labelB); err != nil {
				return err
			}
			sb, err := m.String(b)
			if err != nil {
				return err
			}
			return writeNumeralDisplayPart(w, sb+"\n")
		})
}

func (m *IntegerManager) traceBitwiseOrOperands(a, b *Int) {
	m.traceBitwiseOrPair(1034, "a1: ", "b1: ", a, b)
}

func (m *IntegerManager) traceBitwiseOrChunks(a, b *Int) {
	m.traceBitwiseOrPair(1037, "a2: ", "b2: ", a, b)
}

func (m *IntegerManager) traceBitwiseOrUnsigned(a, b uint64) {
	_ = m.sourceTraceTags().EmitConditional("mpz", "bitwise_or", "src/util/mpz.cpp", 1039,
		func() bool { return true }, func(w io.Writer) error {
			return writeNumeralDisplayPart(w, "uint(a2): "+strconv.FormatUint(a, 10)+
				", uint(b2): "+strconv.FormatUint(b, 10)+"\n")
		})
}

func (m *IntegerManager) traceRootBisection(upper, lower, mid *Int) {
	_ = m.sourceTraceTags().EmitConditional("mpz", "root", "src/util/mpz.cpp", 1987,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range []struct {
				prefix string
				value  *Int
			}{
				{"upper: ", upper},
				{"\nlower: ", lower},
				{"\nmid: ", mid},
			} {
				if err := writeNumeralDisplayPart(w, part.prefix); err != nil {
					return err
				}
				if err := m.DisplayTo(w, part.value); err != nil {
					return err
				}
			}
			return writeNumeralDisplayPart(w, "\n")
		})
}
