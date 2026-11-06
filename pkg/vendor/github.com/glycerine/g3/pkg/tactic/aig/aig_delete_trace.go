package aig

// Source: src/tactic/aig/aig.cpp:149; src/util/trace.h:40-46.

import (
	"io"
)

func (m *Manager) traceDeleting(n *node) {
	tags := m.ast.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("aig_lit_count") {
		return
	}
	_ = tags.EmitConditional("aig_lit_count", "delete_node", "src/tactic/aig/aig.cpp", 149,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "deleting: ")
			writeAIGRef(w, Literal{node: n})
			_, _ = io.WriteString(w, "\n")
			return nil
		})
}
