package protomodel

// Source: retained TRACE sites in src/smt/proto_model/array_factory.cpp.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

type arrayFactoryTraceOutput struct{ io.Writer }

// Native ostream badbit does not stop later TRACE operands. Preserve the
// attempted byte count while the TraceTags owner records header/footer errors.
func (w arrayFactoryTraceOutput) Write(p []byte) (int, error) {
	_, _ = w.Writer.Write(p)
	return len(p), nil
}

func (f *ArrayFactory) trace(tag, function string, line int, condition func() bool, body func(io.Writer) error) error {
	tags := f.manager.BorrowSourceTraceTags()
	if !tags.ShouldTrace(tag) {
		return nil
	}
	if condition == nil {
		condition = func() bool { return true }
	}
	var operandErr error
	err := tags.EmitConditional(tag, function, "src/smt/proto_model/array_factory.cpp", line, condition, func(out io.Writer) error {
		operandErr = body(arrayFactoryTraceOutput{out})
		return operandErr
	})
	if operandErr != nil {
		return operandErr
	}
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func arrayFactoryDisplayAST(manager *ast.ASTManager, out io.Writer, value ast.ASTHandle) error {
	printer, err := ast.NewSimpleASTPrinterContext(manager, out, out)
	if err != nil {
		return err
	}
	kind, err := manager.GetKind(value)
	if err != nil {
		return err
	}
	switch kind {
	case ast.ASTSort:
		return printer.DisplaySort(out, value, 0)
	case ast.ASTFuncDecl:
		return printer.DisplayFuncDecl(out, value, 0)
	default:
		return printer.DisplayExpr(out, value, 0)
	}
}
