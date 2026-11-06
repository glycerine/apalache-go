package rewriter

// Source: src/ast/rewriter/rewriter.cpp:45-59; src/util/trace.h:40-46.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (c *rewriterCore) traceCacheResult(k, v ast.ASTHandle) error {
	tags := c.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("rewriter_cache_result") {
		return nil
	}
	globals := uparams.BorrowProcessGlobalParamsService()
	if globals == nil || globals.Context == nil {
		return uparams.ErrGlobalParamsOwner
	}
	err := tags.EmitConditional("rewriter_cache_result", "cache_result", "src/ast/rewriter/rewriter.cpp", 52,
		func() bool { return true }, func(w io.Writer) error {
			if err := writeBVTraceSMT2(c.manager, w, k, globals); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "\n--->\n"); err != nil {
				return err
			}
			if err := writeBVTraceSMT2(c.manager, w, v, globals); err != nil {
				return err
			}
			_, err := io.WriteString(w, "\n")
			return err
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
