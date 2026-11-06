package rewriter

// Source: src/ast/rewriter/push_app_ite.cpp:77-78.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (c *PushAppITEConfiguration) traceResult(result ast.ASTHandle) error {
	return c.emitPushITETrace("reduce_app", 78, func(w io.Writer, globals *uparams.GlobalParamsService) error {
		if err := c.writePushITEAST(w, result, globals); err != nil {
			return err
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}
