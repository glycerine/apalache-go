package ast

// Source: src/ast/ast.cpp:1782,1790-1793; src/util/trace.h.

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (m *ASTManager) traceDeleteNodeBug(r astNodeRef) {
	tags := m.traceTags
	if tags == nil {
		tags = udiag.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("delete_node_bug") {
		return
	}
	_ = tags.EmitConditional("delete_node_bug", "delete_node", "src/ast/ast.cpp", 1782,
		func() bool { return true }, func(w io.Writer) error {
			out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.BudgetField})
			if err != nil {
				return err
			}
			printErr := m.lowLevelPrint(out, r, true, true)
			rendered, readErr := out.String()
			_, _ = io.WriteString(w, rendered)
			closeErr := out.Close()
			if err := errors.Join(printErr, readErr, closeErr); err != nil {
				return err
			}
			_, _ = io.WriteString(w, "\n")
			return nil
		})
}

func (m *ASTManager) traceDeletingNode(r astNodeRef, n *astNodeRecord) {
	tags := m.traceTags
	if tags == nil {
		tags = udiag.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("ast") && !tags.ShouldTrace("del_quantifier") && !tags.ShouldTrace("mk_var_bug") && !tags.ShouldTrace("ast_delete_node") {
		return
	}
	id := strconv.FormatUint(uint64(n.id), 10)
	pointer := func(w io.Writer) { _, _ = fmt.Fprintf(w, "%p", n) }
	if tags.ShouldTrace("ast") {
		_ = tags.EmitConditional("ast", "delete_node", "src/ast/ast.cpp", 1790,
			func() bool { return true }, func(w io.Writer) error {
				_, _ = io.WriteString(w, "Deleting object ")
				_, _ = io.WriteString(w, id)
				_, _ = io.WriteString(w, " ")
				pointer(w)
				_, _ = io.WriteString(w, "\n")
				return nil
			})
	}
	if tags.ShouldTrace("del_quantifier") {
		_ = tags.EmitConditional("del_quantifier", "delete_node", "src/ast/ast.cpp", 1791,
			func() bool { return n.kind == ASTQuantifier }, func(w io.Writer) error {
				_, _ = io.WriteString(w, "deleting quantifier ")
				_, _ = io.WriteString(w, id)
				_, _ = io.WriteString(w, " ")
				pointer(w)
				_, _ = io.WriteString(w, "\n")
				return nil
			})
	}
	if tags.ShouldTrace("mk_var_bug") {
		_ = tags.EmitConditional("mk_var_bug", "delete_node", "src/ast/ast.cpp", 1792,
			func() bool { return true }, func(w io.Writer) error {
				_, _ = io.WriteString(w, "del_ast: ")
				_, _ = io.WriteString(w, id)
				_, _ = io.WriteString(w, "\n")
				return nil
			})
	}
	if tags.ShouldTrace("ast_delete_node") {
		_ = tags.EmitConditional("ast_delete_node", "delete_node", "src/ast/ast.cpp", 1793,
			func() bool { return true }, func(w io.Writer) error {
				out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.BudgetField})
				if err != nil {
					return err
				}
				printErr := m.boundedLowLevelPrint(out, r, 3)
				rendered, readErr := out.String()
				_, _ = io.WriteString(w, rendered)
				closeErr := out.Close()
				if err := errors.Join(printErr, readErr, closeErr); err != nil {
					return err
				}
				_, _ = io.WriteString(w, "\n")
				return nil
			})
	}
}
