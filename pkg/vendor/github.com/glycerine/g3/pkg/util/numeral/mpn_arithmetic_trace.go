package numeral

// Source: src/util/mpn.cpp:43-57,65-143,157-204,406-429.

import (
	"io"
)

// Source trace() uses a lexical 4096-byte buffer and the same to_string
// routine as ordinary decimal output. Keep a separate formatting scope so
// nested traces cannot reuse an active caller's lexical conversion buffers.
func (m *NaturalManager) writeMPNTraceValue(w io.Writer, a limbView) error {
	var buf [4096]byte
	dst := sourceDecimalDestination{external: buf[:]}
	var scope naturalFormattingScope
	if err := m.sourceToCStringScoped(a, dst, uint64(len(buf)), &scope); err != nil {
		return err
	}
	for i, c := range buf {
		if c == 0 {
			return writeNumeralDisplayPart(w, string(buf[:i]))
		}
	}
	return nil
}

func (m *NaturalManager) traceMPNBinary(a, b []uint32, op string) {
	m.traceMPNBinaryView(limbView{external: a, size: uint32(len(a))}, limbView{external: b, size: uint32(len(b))}, op)
}

func (m *NaturalManager) traceMPNBinaryView(a, b limbView, op string) {
	if !SourceExtendedGCDDebug {
		return
	}
	tags := m.sourceTraceTags()
	_ = tags.EmitConditional("mpn", "trace", "src/util/mpn.cpp", 411,
		func() bool { return true }, func(w io.Writer) error {
			if err := writeNumeralDisplayPart(w, "[mpn] "); err != nil {
				return err
			}
			if err := m.writeMPNTraceValue(w, a); err != nil {
				return err
			}
			if err := writeNumeralDisplayPart(w, " "+op+" "); err != nil {
				return err
			}
			if err := m.writeMPNTraceValue(w, b); err != nil {
				return err
			}
			return writeNumeralDisplayPart(w, " == ")
		})
}

func (m *NaturalManager) traceMPNUnary(a []uint32, line int, newline bool) {
	m.traceMPNUnaryView(limbView{external: a, size: uint32(len(a))}, line, newline)
}

func (m *NaturalManager) traceMPNUnaryView(a limbView, line int, newline bool) {
	if !SourceExtendedGCDDebug {
		return
	}
	function := "trace"
	if newline {
		function = "trace_nl"
	}
	_ = m.sourceTraceTags().EmitConditional("mpn", function, "src/util/mpn.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			if err := m.writeMPNTraceValue(w, a); err != nil {
				return err
			}
			if newline {
				return writeNumeralDisplayPart(w, "\n")
			}
			return nil
		})
}

func (m *NaturalManager) traceMPNCompareResult(result int) {
	body := " == "
	if result > 0 {
		body = " > "
	} else if result < 0 {
		body = " < "
	}
	_ = m.sourceTraceTags().EmitConditional("mpn", "compare", "src/util/mpn.cpp", 55,
		func() bool { return true }, func(w io.Writer) error { return writeNumeralDisplayPart(w, body) })
}

func (m *NaturalManager) traceMPNDivisionOutputs(a, b, q, r limbView) {
	m.traceMPNUnaryView(q, 427, true)
	m.traceMPNBinaryView(a, b, "%")
	m.traceMPNUnaryView(r, 427, true)
}
