package rewriter

// Source: src/ast/rewriter/var_subst.cpp:207-211.

import (
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"io"
)

func traceUnusedVarsResult(m *ast.ASTManager, q ast.ASTHandle, out ast.ASTRef) error {
	result, err := m.RefGet(out)
	if err != nil {
		return err
	}
	return emitUnusedVarsTrace(m, "elim_unused_vars", 210, func(w io.Writer, globals *uparams.GlobalParamsService) error {
		if err := writeUnusedVarsAST(m, w, q, globals); err != nil {
			return err
		}
		if _, err := io.WriteString(w, " -> "); err != nil {
			return err
		}
		if err := writeUnusedVarsAST(m, w, result, globals); err != nil {
			return err
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}
