package rewriter

// Source: src/ast/rewriter/push_app_ite.cpp:53-55; src/ast/ast_pp.h.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (c *PushAppITEConfiguration) writePushITEAST(w io.Writer, h ast.ASTHandle, globals *uparams.GlobalParamsService) error {
	return writeBVTraceSMT2(c.manager, w, h, globals)
}

func (c *PushAppITEConfiguration) writePushITEDeclName(w io.Writer, decl ast.ASTHandle) error {
	symbol, err := c.manager.GetASTName(decl)
	if err != nil {
		return err
	}
	name, err := c.manager.Symbols.String(symbol)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, name)
	return err
}
