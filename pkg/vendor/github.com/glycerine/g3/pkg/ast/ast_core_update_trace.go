package ast

// Source: src/ast/ast.cpp:2454; retained util/trace TRACE call site.

import (
	"io"
	"strconv"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *ASTManager) traceQuantifierWeight(id uint32, oldWeight, newWeight int32) {
	tags := m.traceTags
	if tags == nil {
		tags = udiag.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("update_quantifier_weight", "update_quantifier_weight", "src/ast/ast.cpp", 2454,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "#")
			_, _ = io.WriteString(w, strconv.FormatUint(uint64(id), 10))
			_, _ = io.WriteString(w, " ")
			_, _ = io.WriteString(w, strconv.FormatInt(int64(oldWeight), 10))
			_, _ = io.WriteString(w, " -> ")
			_, _ = io.WriteString(w, strconv.FormatInt(int64(newWeight), 10))
			_, _ = io.WriteString(w, "\n")
			return nil
		})
}
