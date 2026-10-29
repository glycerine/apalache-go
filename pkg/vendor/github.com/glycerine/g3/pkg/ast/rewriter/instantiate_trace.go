package rewriter

// Source: src/ast/rewriter/var_subst.cpp:164,168-173.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func emitInstantiationTrace(m *ast.ASTManager, tag, function string, line int, body func(io.Writer, *uparams.GlobalParamsService) error) error {
	tags := m.BorrowMemoryTraceTags()
	if !tags.ShouldTrace(tag) {
		return nil
	}
	globals := uparams.BorrowProcessGlobalParamsService()
	if globals == nil || globals.Context == nil {
		return uparams.ErrGlobalParamsOwner
	}
	err := tags.EmitConditional(tag, function, "src/ast/rewriter/var_subst.cpp", line,
		func() bool { return true }, func(w io.Writer) error { return body(w, globals) })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func traceInstantiationSubstitution(m *ast.ASTManager, q, rewritten ast.ASTHandle) error {
	return emitInstantiationTrace(m, "var_subst", "instantiate", 164, func(w io.Writer, g *uparams.GlobalParamsService) error {
		if e := writeBVTraceSMT2(m, w, q, g); e != nil {
			return e
		}
		if _, e := io.WriteString(w, "\n"); e != nil {
			return e
		}
		if e := writeBVTraceSMT2(m, w, rewritten, g); e != nil {
			return e
		}
		_, e := io.WriteString(w, "\n")
		return e
	})
}

func traceInstantiationResult(m *ast.ASTManager, q ast.ASTHandle, values []ast.ASTHandle, result ast.ASTHandle) error {
	return emitInstantiationTrace(m, "instantiate_bug", "instantiate", 168, func(w io.Writer, g *uparams.GlobalParamsService) error {
		if e := writeBVTraceSMT2(m, w, q, g); e != nil {
			return e
		}
		if _, e := io.WriteString(w, "\nusing\n"); e != nil {
			return e
		}
		for _, value := range values {
			if e := writeBVTraceSMT2(m, w, value, g); e != nil {
				return e
			}
			if _, e := io.WriteString(w, "\n"); e != nil {
				return e
			}
		}
		if _, e := io.WriteString(w, "\n----->\n"); e != nil {
			return e
		}
		if e := writeBVTraceSMT2(m, w, result, g); e != nil {
			return e
		}
		_, e := io.WriteString(w, "\n")
		return e
	})
}
