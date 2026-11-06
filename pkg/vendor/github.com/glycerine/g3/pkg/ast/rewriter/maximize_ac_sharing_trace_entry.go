package rewriter

// Source: src/ast/rewriter/maximize_ac_sharing.cpp:49,54.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (c *MaximizeBVSharingConfiguration) traceBeforeReuse(count int) error {
	return c.emitSharingTrace("reduce_app", 49, func(w io.Writer, _ *uparams.GlobalParamsService) error {
		_, err := io.WriteString(w, "before-reuse: num_args: "+strconv.Itoa(count)+"\n")
		return err
	})
}

func (c *MaximizeBVSharingConfiguration) traceArguments(args []ast.ASTHandle) error {
	return c.emitSharingTrace("reduce_app", 54, func(w io.Writer, g *uparams.GlobalParamsService) error {
		if _, err := io.WriteString(w, "args: "); err != nil {
			return err
		}
		for _, h := range args {
			if err := c.writeSharingAST(w, h, g); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		return nil
	})
}
