package rewriter

// Source: src/ast/rewriter/der.cpp:140,229.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (d *DER) traceQuantifier(q ast.ASTHandle) error {
	return d.emitDERTrace("der", "operator()", 140, func(w io.Writer, globals *uparams.GlobalParamsService) error {
		return d.writeDERASTLine(w, q, globals)
	})
}

func (d *DER) traceNoCandidate(q ast.ASTHandle) error {
	return d.emitDERTrace("der_bug", "reduce1", 229, func(w io.Writer, globals *uparams.GlobalParamsService) error {
		if _, err := io.WriteString(w, "Did not find any diseq\n"); err != nil {
			return err
		}
		return d.writeDERASTLine(w, q, globals)
	})
}
