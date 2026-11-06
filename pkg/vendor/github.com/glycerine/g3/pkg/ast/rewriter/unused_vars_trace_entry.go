package rewriter

// Source: src/ast/rewriter/var_subst.cpp:98.

import (
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"io"
)

func traceUnusedVarsEntry(m *ast.ASTManager, q ast.ASTHandle) error {
	return emitUnusedVarsTrace(m, "operator()", 98, func(w io.Writer, globals *uparams.GlobalParamsService) error {
		if err := writeUnusedVarsAST(m, w, q, globals); err != nil {
			return err
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}
