package aig

// Source: src/tactic/aig/aig.cpp:1131-1200,1261,1284;
// src/util/trace.h:40-46.

import (
	"io"
	"strconv"
)

func (m *Manager) traceSharingTry(a, b, c Literal, left, second bool) {
	tags := m.ast.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("max_sharing") {
		return
	}
	line := 1131
	action := "improve_sharing_left"
	if left && second {
		line = 1149
	} else if !left && !second {
		line = 1176
	} else if !left {
		line = 1195
	}
	if !left {
		action = "improve_sharing_right"
	}
	_ = tags.EmitConditional("max_sharing", action, "src/tactic/aig/aig.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "trying (and ")
			if !left {
				_, _ = io.WriteString(w, "(and ")
			}
			writeAIGRef(w, a)
			if left {
				_, _ = io.WriteString(w, " (and ")
			} else {
				_, _ = io.WriteString(w, " ")
			}
			if second {
				writeAIGRef(w, c)
			} else {
				writeAIGRef(w, b)
			}
			if left {
				_, _ = io.WriteString(w, " ")
			} else {
				_, _ = io.WriteString(w, ") ")
			}
			if second {
				writeAIGRef(w, b)
			} else {
				writeAIGRef(w, c)
			}
			if left {
				_, _ = io.WriteString(w, "))\n")
			} else {
				_, _ = io.WriteString(w, ")\n")
			}
			return nil
		})
}

func (m *Manager) traceSharingProcessing(n *node, index uint8) {
	tags := m.ast.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("max_sharing") {
		return
	}
	_ = tags.EmitConditional("max_sharing", "process", "src/tactic/aig/aig.cpp", 1261,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "processing ")
			writeAIGRef(w, Literal{node: n})
			_, _ = io.WriteString(w, " idx: ")
			_, _ = io.WriteString(w, strconv.FormatUint(uint64(index), 10))
			_, _ = io.WriteString(w, "\n")
			return nil
		})
}

func (m *Manager) traceSharingNull(null bool) {
	tags := m.ast.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("max_sharing") {
		return
	}
	_ = tags.EmitConditional("max_sharing", "operator()", "src/tactic/aig/aig.cpp", 1284,
		func() bool { return true }, func(w io.Writer) error {
			value := "0"
			if null {
				value = "1"
			}
			_, _ = io.WriteString(w, "r.is_null(): "+value+"\n")
			return nil
		})
}

func (m *Manager) traceSharingResult(l Literal) error {
	tags := m.ast.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("max_sharing") {
		return nil
	}
	return tags.EmitConditional("max_sharing", "operator()", "src/tactic/aig/aig.cpp", 1295,
		func() bool { return true }, func(w io.Writer) error {
			if _, err := io.WriteString(w, "result:\n"); err != nil {
				return err
			}
			return m.writeAIGDisplay(w, l)
		})
}

func (m *Manager) traceSharingImproved(old *node, replacement Literal, left, second bool) error {
	tags := m.ast.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("max_sharing") {
		return nil
	}
	line, action := 1144, "improve_sharing_left"
	if left && second {
		line = 1162
	} else if !left {
		line, action = 1189, "improve_sharing_right"
		if second {
			line = 1207
		}
	}
	return tags.EmitConditional("max_sharing", action, "src/tactic/aig/aig.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			if _, err := io.WriteString(w, "improved:\n"); err != nil {
				return err
			}
			if err := m.writeAIGDisplay(w, Literal{node: old}); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "---->\n"); err != nil {
				return err
			}
			return m.writeAIGDisplay(w, replacement)
		})
}
