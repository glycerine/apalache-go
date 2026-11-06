package aig

// Source: src/tactic/aig/aig.cpp:1341-1351; src/util/trace.h:40-46.

import "io"

func (m *Manager) traceMkAnd(left, right, result Literal) error {
	tags := m.ast.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("mk_and_bug") {
		return nil
	}
	return tags.EmitConditional("mk_and_bug", "mk_and", "src/tactic/aig/aig.cpp", 1343,
		func() bool { return true }, func(w io.Writer) error {
			if err := m.writeAIGDisplay(w, left); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "AND\n"); err != nil {
				return err
			}
			if err := m.writeAIGDisplay(w, right); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "-->\n"); err != nil {
				return err
			}
			if err := m.writeAIGDisplay(w, result); err != nil {
				return err
			}
			_, err := io.WriteString(w, "\n")
			return err
		})
}
