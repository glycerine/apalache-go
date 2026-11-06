package rewriter

// Source: src/ast/rewriter/var_subst.cpp expr_ref trace formatting.

import (
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"io"
)

func writeUnusedVarsAST(m *ast.ASTManager, w io.Writer, h ast.ASTHandle, globals *uparams.GlobalParamsService) error {
	return writeBVTraceSMT2(m, w, h, globals)
}
