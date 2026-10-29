package numeral

// Source: src/util/mpfx.cpp:647-648,745-756.

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"strconv"
)

func (m *FixedPointManager) fixedPowerTraceInput(a *FixedPoint) string {
	if !m.fixedTraceTags().ShouldTrace("mpfx_trace") {
		return ""
	}
	var b bytes.Buffer
	_ = m.Display(&b, a)
	return b.String()
}

func (m *FixedPointManager) traceFixedPower(input string, exponent uint32, out *FixedPoint) {
	tags := m.fixedTraceTags()
	if tags.ShouldTrace("mpfx_trace") {
		_ = tags.EmitShort("mpfx_trace", func(w io.Writer) error {
			bound := ">="
			if m.roundUp {
				bound = "<="
			}
			if err := writeFixedPointPart(w, "[mpfx] ("+input+") ^ "+strconv.FormatUint(uint64(exponent), 10)+bound); err != nil {
				return err
			}
			if err := m.Display(w, out); err != nil {
				return err
			}
			return writeFixedPointPart(w, "\n")
		})
	}
	if tags.ShouldTrace("mpfx_power") {
		_ = tags.EmitConditional("mpfx_power", "power", "src/util/mpfx.cpp", 648,
			func() bool { return true }, func(w io.Writer) error {
				if err := m.displayFixedRawTrace(w, out); err != nil {
					return err
				}
				return writeFixedPointPart(w, "\n")
			})
	}
}

func (m *FixedPointManager) displayFixedRawTrace(w io.Writer, n *FixedPoint) error {
	if n.sourceNegative() {
		if err := writeFixedPointPart(w, "-"); err != nil {
			return err
		}
	}
	var magnitude, word big.Int
	magnitude.Abs(&n.scaled)
	for i := int(m.intWords + m.fracWords); i > 0; {
		if uint32(i) == m.fracWords {
			if err := writeFixedPointPart(w, "."); err != nil {
				return err
			}
		}
		i--
		word.Rsh(&magnitude, uint(i)*32)
		if err := writeFixedPointPart(w, fmt.Sprintf("%08x", uint32(word.Uint64()))); err != nil {
			return err
		}
	}
	return nil
}
