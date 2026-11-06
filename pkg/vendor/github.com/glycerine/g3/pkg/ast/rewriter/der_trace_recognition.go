package rewriter

// Source: src/ast/rewriter/der.cpp:51,100.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (d *DER) traceRecognized(function string, line int, literal ast.ASTHandle) error {
	return d.emitDERTrace("der", function, line, func(w io.Writer, globals *uparams.GlobalParamsService) error {
		return d.writeDERASTLine(w, literal, globals)
	})
}
