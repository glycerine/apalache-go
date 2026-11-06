package rewriter

// Source: src/ast/rewriter/bv_bounds.cpp TRACE("bv_bounds") sites.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (b *BVBounds) traceText(function string, line int, text string) error {
	tags := b.manager.BorrowMemoryTraceTags()
	return tags.EmitConditional("bv_bounds", function, "src/ast/rewriter/bv_bounds.cpp", line, func() bool { return true }, func(w io.Writer) error {
		_, err := io.WriteString(w, text+"\n")
		return err
	})
}

func (b *BVBounds) traceValueIf(condition bool, function string, line int, prefix string, value ast.ASTHandle, suffix string) error {
	if !condition {
		return nil
	}
	return b.traceValue(function, line, prefix, value, suffix)
}

func (b *BVBounds) traceValue(function string, line int, prefix string, value ast.ASTHandle, suffix string) error {
	tags := b.manager.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("bv_bounds") {
		return nil
	}
	printer, err := ast.NewLegacySMTPrinter(b.manager)
	if err != nil {
		return err
	}
	return tags.EmitConditional("bv_bounds", function, "src/ast/rewriter/bv_bounds.cpp", line, func() bool { return true }, func(w io.Writer) error {
		if _, err := io.WriteString(w, prefix); err != nil {
			return err
		}
		if err := printer.DisplayExpr(w, value, nil); err != nil {
			return err
		}
		_, err := io.WriteString(w, suffix+"\n")
		return err
	})
}
