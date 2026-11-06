//go:build g3debug

package numeral

// Source: src/util/mpbq.cpp:167,195, reached only with MPBQ_DEBUG.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *BinaryRationalManager) traceMPBQArithmetic(a, b, result, expected *OwnedRational, line int) {
	tags := m.Z.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	name := "add"
	if line == 195 {
		name = "sub"
	}
	_ = tags.EmitConditional("mpbq_bug", name, "src/util/mpbq.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			if err := writeNumeralDisplayPart(w, name+" a: "); err != nil {
				return err
			}
			for i, v := range []*OwnedRational{a, b, result, expected} {
				if i != 0 {
					part := ", b: "
					if i == 2 {
						part = ", r: "
					} else if i == 3 {
						part = ", expected: "
					}
					if err := writeNumeralDisplayPart(w, part); err != nil {
						return err
					}
				}
				value, err := m.Diagnostics.String(v)
				if err != nil {
					return err
				}
				if err := writeNumeralDisplayPart(w, value); err != nil {
					return err
				}
			}
			return writeNumeralDisplayPart(w, "\n")
		})
}
