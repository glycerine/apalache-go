package model

// Source: model_implicant::eval_eq, model_implicant.cpp:625-637;
// model_implicant::eval_array_eq, model_implicant.cpp:540-541,591-592;
// model_implicant::check_model, model_implicant.cpp:844-847;
// model::eval, model.cpp:79-90.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (i *ModelImplicant) evaluateModelWithoutCompletion(expression ast.ASTHandle, out ast.ASTRef) (resultErr error) {
	evaluator, err := NewModelEvaluator(i.model.Core(), i.services)
	if evaluator != nil {
		defer func() { resultErr = errors.Join(resultErr, i.closeTemporaryEvaluator(evaluator)) }()
	}
	if err != nil {
		return err
	}
	if err = evaluator.SetModelCompletion(false); err != nil {
		return err
	}
	err = evaluator.Evaluate(expression, out)
	if err == nil || !rewriter.IsRewriterException(err) {
		return err
	}
	// model::eval catches the typed exception. Its fresh evaluator is closed
	// before the next fallback, so interrupted traversal state cannot escape.
	i.traceModelEvaluationFailure(err)
	return nil
}

func (i *ModelImplicant) traceModelEvaluationFailure(failure error) {
	tags := i.services.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("model_evaluator", "eval", "src/model/model.cpp", 88, func() bool { return true }, func(w io.Writer) error { _, err := io.WriteString(w, failure.Error()+"\n"); return err })
}

func (i *ModelImplicant) traceUnknownEquality(value ast.ASTHandle) error {
	tags := i.services.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("pdr") {
		return nil
	}
	var operandErr error
	_ = tags.EmitConditional("pdr", "eval_eq", "src/model/model_implicant.cpp", 635, func() bool { return true }, func(w io.Writer) error {
		_, _ = io.WriteString(w, "cannot evaluate: ")
		if value.IsNull() {
			_, _ = io.WriteString(w, "null")
		} else {
			operandErr = i.evaluator.writeSMT2(w, value)
			if operandErr != nil {
				return operandErr
			}
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	})
	return operandErr
}

// A fresh model::eval evaluator can stop partway through destruction. Keep
// its remaining fields owned until the implicant completes reset or Close.
func (i *ModelImplicant) closeTemporaryEvaluator(evaluator *ModelEvaluator) error {
	err := evaluator.Close()
	if err != nil && !evaluator.closed {
		i.pendingEvaluators = append(i.pendingEvaluators, evaluator)
	}
	return err
}
