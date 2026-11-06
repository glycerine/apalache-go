package rewriter

// Source: src/ast/rewriter/push_app_ite.cpp:51-55.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (c *PushAppITEConfiguration) traceTarget(decl ast.ASTHandle, args ast.AppArguments) error {
	return c.emitPushITETrace("is_target", 51, func(w io.Writer, globals *uparams.GlobalParamsService) error {
		if _, err := io.WriteString(w, "found target for push app ite:\n"); err != nil {
			return err
		}
		if err := c.writePushITEDeclName(w, decl); err != nil {
			return err
		}
		for i := uint32(0); i < args.Count(); i++ {
			h, err := args.At(i)
			if err != nil {
				return err
			}
			if _, err = io.WriteString(w, " "); err != nil {
				return err
			}
			if err = c.writePushITEAST(w, h, globals); err != nil {
				return err
			}
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}
