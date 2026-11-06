package rewriter

// Source: src/ast/rewriter/der.cpp mk_pp diagnostic bodies.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (d *DER) writeDERAST(w io.Writer, h ast.ASTHandle, globals *uparams.GlobalParamsService) error {
	return writeBVTraceSMT2(d.manager, w, h, globals)
}

func (d *DER) writeDERASTLine(w io.Writer, h ast.ASTHandle, globals *uparams.GlobalParamsService) error {
	if err := d.writeDERAST(w, h, globals); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}
