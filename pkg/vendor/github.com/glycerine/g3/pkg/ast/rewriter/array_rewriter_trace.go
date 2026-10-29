package rewriter

// Source: src/ast/rewriter/array_rewriter.cpp:38-42; src/util/trace.h:40-46.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (rw *ArrayRewriter) writeAppTraceNode(w io.Writer, h ast.ASTHandle, decl bool) (result error) {
	var format ast.ASTHandle
	var err error
	if decl {
		format, err = rw.manager.SMT2PPDeclareFun(h)
	} else {
		format, err = rw.manager.SMT2PPExprTree(h, nil)
	}
	if err != nil {
		return err
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: rw.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	if err = rw.manager.PrintFormat(stream, format, nil, nil, nil); err != nil {
		return err
	}
	body, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, body)
	return err
}

func (rw *ArrayRewriter) traceAppCore(decl ast.ASTHandle, args ast.AppArguments) error {
	tags := rw.manager.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("array_rewriter") {
		return nil
	}
	err := tags.EmitConditional("array_rewriter", "mk_app_core", "src/ast/rewriter/array_rewriter.cpp", 38,
		func() bool { return true }, func(w io.Writer) error {
			if err := rw.writeAppTraceNode(w, decl, true); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
			for i := uint32(0); i < args.Count(); i++ {
				h, err := args.At(i)
				if err != nil {
					return err
				}
				if err = rw.writeAppTraceNode(w, h, false); err != nil {
					return err
				}
				if _, err = io.WriteString(w, "\n"); err != nil {
					return err
				}
			}
			return nil
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
