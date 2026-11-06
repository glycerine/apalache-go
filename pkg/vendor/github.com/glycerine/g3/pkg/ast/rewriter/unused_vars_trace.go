package rewriter

// Source: src/ast/rewriter/var_subst.cpp:98,210; src/util/trace.h.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func emitUnusedVarsTrace(m *ast.ASTManager, function string, line int, body func(io.Writer, *uparams.GlobalParamsService) error) error {
	tags := m.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("elim_unused_vars") {
		return nil
	}
	globals := uparams.BorrowProcessGlobalParamsService()
	if globals == nil || globals.Context == nil {
		return nil
	}
	return emitRewriterSourceTrace(tags, "elim_unused_vars", function, "src/ast/rewriter/var_subst.cpp", line,
		func(w io.Writer) error { return body(w, globals) })
}
