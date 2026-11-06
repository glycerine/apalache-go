package rewriter

// Source: src/ast/rewriter/maximize_ac_sharing.cpp:95.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (c *MaximizeBVSharingConfiguration) traceResult(result ast.ASTHandle) error {
	return c.emitSharingTrace("reduce_app", 95, func(w io.Writer, g *uparams.GlobalParamsService) error {
		if _, err := io.WriteString(w, "result: "); err != nil {
			return err
		}
		if err := c.writeSharingAST(w, result, g); err != nil {
			return err
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}
