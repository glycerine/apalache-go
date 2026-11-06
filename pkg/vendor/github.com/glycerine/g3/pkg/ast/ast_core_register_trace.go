package ast

// Source: src/ast/ast.cpp:1696-1697; src/util/trace.h.

import (
	"io"
	"strconv"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *ASTManager) traceRegisteredNode(id uint32) {
	tags := m.traceTags
	if tags == nil {
		tags = udiag.BorrowProcessTraceTags()
	}
	write := func(tag string, line int, prefix, suffix string) {
		_ = tags.EmitConditional(tag, "register_node_core", "src/ast/ast.cpp", line,
			func() bool { return true }, func(w io.Writer) error {
				_, _ = io.WriteString(w, prefix)
				_, _ = io.WriteString(w, strconv.FormatUint(uint64(id), 10))
				_, _ = io.WriteString(w, suffix)
				return nil
			})
	}
	write("ast", 1696, "Object ", " was created.\n")
	write("mk_var_bug", 1697, "mk_ast: ", "\n")
}
