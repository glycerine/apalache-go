package aig

// Source: src/tactic/aig/aig.cpp:770,863,881,931;
// src/util/trace.h:40-46.

import "io"

func (m *Manager) traceInvalidGoalCache(n *node) {
	tags := m.ast.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("aig2expr") {
		return
	}
	_ = tags.EmitConditional("aig2expr", "get_cached", "src/tactic/aig/aig.cpp", 770,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "invalid get_cached for ")
			writeAIGRef(w, Literal{node: n})
			_, _ = io.WriteString(w, "\n")
			return nil
		})
}

func (m *Manager) traceGoalConversion(n *node, line int, label string) {
	tags := m.ast.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("aig2expr") {
		return
	}
	_ = tags.EmitConditional("aig2expr", "aig2expr", "src/tactic/aig/aig.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, label)
			writeAIGRef(w, Literal{node: n})
			_, _ = io.WriteString(w, "\n")
			return nil
		})
}
