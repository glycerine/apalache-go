package rewriter

// Source: src/ast/rewriter/bool_rewriter.cpp:594-595,665-671; src/util/trace.h:40-46.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// C++ prints raw expr* addresses on its second line. The concrete Go AST
// owner instead exposes the stable typed identity of each live handle.
func boolTraceASTIdentity(h ast.ASTHandle) string {
	return strconv.FormatUint(h.Origin(), 10) + ":" +
		strconv.FormatUint(uint64(h.Slot()), 10) + ":" +
		strconv.FormatUint(uint64(h.Generation()), 10)
}

func (rw *boolRewriter) traceTryITEValueFalse(t, f, val ast.ASTHandle) error {
	tags := rw.manager.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("try_ite_value") {
		return nil
	}
	globals := uparams.BorrowProcessGlobalParamsService()
	if globals == nil || globals.Context == nil {
		return uparams.ErrGlobalParamsOwner
	}
	err := tags.EmitConditional("try_ite_value", "try_ite_value", "src/ast/rewriter/bool_rewriter.cpp", 594,
		func() bool { return true }, func(w io.Writer) error {
			for i, h := range [...]ast.ASTHandle{t, f, val} {
				if i != 0 {
					if _, err := io.WriteString(w, " "); err != nil {
						return err
					}
				}
				if err := writeBVTraceSMT2(rw.manager, w, h, globals); err != nil {
					return err
				}
			}
			_, err := io.WriteString(w, "\n"+boolTraceASTIdentity(t)+" "+boolTraceASTIdentity(f)+" "+boolTraceASTIdentity(val)+"\n")
			return err
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (rw *boolRewriter) writeBoundedTrace(w io.Writer, h ast.ASTHandle) (result error) {
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: rw.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	if err = rw.manager.DisplayBoundedLowLevel(stream, h, 3); err != nil {
		return err
	}
	body, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, body)
	return err
}

func (rw *boolRewriter) traceITEValueResult(lhs, rhs ast.ASTHandle, out ast.ASTRef, status brStatus, line int) error {
	tags := rw.manager.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("try_ite_value") || status == brFailed {
		return nil
	}
	result, err := rw.manager.RefGet(out)
	if err != nil {
		return err
	}
	err = tags.EmitConditional("try_ite_value", "mk_eq_core", "src/ast/rewriter/bool_rewriter.cpp", line,
		func() bool { return status != brFailed }, func(w io.Writer) error {
			for i, h := range [...]ast.ASTHandle{lhs, rhs, result} {
				if i != 0 {
					separator := "\n"
					if i == 2 {
						separator = "\n--->\n"
					}
					if _, err := io.WriteString(w, separator); err != nil {
						return err
					}
				}
				if err := rw.writeBoundedTrace(w, h); err != nil {
					return err
				}
			}
			_, err := io.WriteString(w, "\n")
			return err
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
