package model

// Source: src/model/model_implicant.cpp:850-868.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (i *ModelImplicant) evaluationTruth(expression ast.ASTHandle) string {
	if i.isTrue(expression) {
		return "true"
	}
	if i.isFalse(expression) {
		return "false"
	}
	return "unknown"
}

func (i *ModelImplicant) reportEvaluation(level uint32, prefix string, expression ast.ASTHandle, suffix string) error {
	enabled, err := diagnostic.ProcessVerboseEnabled(level)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = out.AppendCString(prefix); err != nil {
		return err
	}
	format, err := i.manager.SMT2PPExprTree(expression, nil)
	if err != nil {
		return err
	}
	empty := i.services.Context.EmptyValue()
	if err = i.manager.PrintFormat(out, format, i.services.Context, &empty, i.services.Globals); err != nil {
		return err
	}
	return out.AppendCString(suffix)
}

func (i *ModelImplicant) traceUndeterminedModel() error {
	tags := i.services.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("pdr") {
		return nil
	}
	var operandErr error
	_ = tags.EmitConditional("pdr", "check_model", "src/model/model_implicant.cpp", 868, func() bool { return true }, func(w io.Writer) error {
		operandErr = i.model.Core().WriteSMT2(w, i.services.Context, i.services.Globals, 0)
		return operandErr
	})
	if tags.IsStreamFailure(operandErr) {
		return nil
	}
	return operandErr
}

// Source: model_implicant.cpp:250,259,431,807, before UNREACHABLE.
func (i *ModelImplicant) unhandledTerm(expression ast.ASTHandle, line int32) error {
	if err := i.reportEvaluation(0, "Term not handled ", expression, "\n"); err != nil {
		return err
	}
	return diagnostic.BorrowProcessDebugContext().UnreachableSource("src/model/model_implicant.cpp", line)
}
