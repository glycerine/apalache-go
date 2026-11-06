package spacer

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

// spacerModelEvaluatorConstructionFailure keeps a partially initialized
// evaluator reachable when its cleanup is refused. This is the concrete Go
// owner corresponding to model_evaluator_util's m_mev pointer during reset.
type spacerModelEvaluatorConstructionFailure struct {
	evaluator         *model.ModelEvaluator
	runtime           *memory.Runtime
	cause, cleanupErr error
	reported          error
	reportText        string
}

// spacerModelValueOwner keeps EvaluateValue's owning result reachable until
// the Boolean probe has completed its native release frontier.
type spacerModelValueOwner struct {
	manager *ast.ASTManager
	value   ast.ASTRef
}

func (o *spacerModelValueOwner) close() error {
	if o.value == (ast.ASTRef{}) {
		return nil
	}
	return o.manager.DelRef(&o.value)
}

type spacerModelValueCleanupFailure struct {
	owner             *spacerModelValueOwner
	original, cleanup error
	reported          error
	text              string
}

var _ memory.ConstructionFailure = (*spacerModelValueCleanupFailure)(nil)

func (f *spacerModelValueCleanupFailure) Error() string        { return f.text }
func (f *spacerModelValueCleanupFailure) Unwrap() error        { return f.reported }
func (f *spacerModelValueCleanupFailure) OriginalCause() error { return f.original }
func (f *spacerModelValueCleanupFailure) CleanupCause() error  { return f.cleanup }
func (f *spacerModelValueCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *spacerModelValueCleanupFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}

func finishSpacerModelValue(owner *spacerModelValueOwner, cause error) error {
	cleanup := owner.close()
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &spacerModelValueCleanupFailure{
		owner: owner, original: original, cleanup: cleanup,
		reported: reported, text: reported.Error(),
	}
}

func (f *spacerModelEvaluatorConstructionFailure) Error() string {
	return f.reportText
}
func (f *spacerModelEvaluatorConstructionFailure) Unwrap() error {
	return f.reported
}
func (f *spacerModelEvaluatorConstructionFailure) Cleanup() error {
	if f.evaluator == nil {
		f.cleanupErr = nil
		return nil
	}
	f.cleanupErr = f.evaluator.Close()
	if f.cleanupErr == nil {
		f.evaluator = nil
	}
	return f.cleanupErr
}
func (f *spacerModelEvaluatorConstructionFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *spacerModelEvaluatorConstructionFailure) OriginalCause() error          { return f.cause }
func (f *spacerModelEvaluatorConstructionFailure) CleanupCause() error           { return f.cleanupErr }

func closeFailedSpacerModelEvaluator(evaluator *model.ModelEvaluator, config model.ModelEvaluatorConfig, cause error) (*model.ModelEvaluator, error) {
	if evaluator == nil {
		return nil, cause
	}
	if cleanup := evaluator.Close(); cleanup != nil {
		reported := errors.Join(cause, cleanup)
		return nil, &spacerModelEvaluatorConstructionFailure{
			evaluator: evaluator, runtime: config.Memory, cause: cause, cleanupErr: cleanup,
			reported: reported, reportText: reported.Error(),
		}
	}
	return nil, cause
}

// newSpacerModelEvaluator consumes partial construction owners before returning
// an error, so callers only own an evaluator after successful construction.
func newSpacerModelEvaluator(md *model.ModelCore, config model.ModelEvaluatorConfig) (*model.ModelEvaluator, error) {
	evaluator, err := model.NewModelEvaluator(md, config)
	if err != nil {
		return closeFailedSpacerModelEvaluator(evaluator, config, err)
	}
	return evaluator, nil
}

// Source: spacer_util.cpp model_evaluator_util::eval's typed exception catch.
func traceSpacerModelEvaluationFailure(manager *ast.ASTManager, config model.ModelEvaluatorConfig, failure error) error {
	tags := config.Trace
	if tags == nil {
		tags = manager.BorrowSourceTraceTags()
	}
	err := tags.EmitConditional("spacer_model_evaluator", "eval", "src/muz/spacer/spacer_util.cpp", 101,
		func() bool { return true }, func(out io.Writer) error { _, err := io.WriteString(out, failure.Error()+"\n"); return err })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

// evaluateSpacerModelBoolean implements model_evaluator_util's Boolean probes:
// evaluator rewrite exceptions mean unknown; ownership/parameter errors remain
// errors. An interrupted evaluator still obeys the rewriter resume contract.
func evaluateSpacerModelBoolean(manager *ast.ASTManager, evaluator *model.ModelEvaluator, config model.ModelEvaluatorConfig, expression ast.ASTHandle) (truth, falsehood bool, resultErr error) {
	if err := evaluator.SetModelCompletion(false); err != nil {
		return false, false, err
	}
	value, err := evaluator.EvaluateValue(expression)
	if err != nil {
		if rewriter.IsRewriterException(err) {
			return false, false, traceSpacerModelEvaluationFailure(manager, config, err)
		}
		return false, false, err
	}
	owner := &spacerModelValueOwner{manager: manager, value: value}
	defer func() {
		resultErr = finishSpacerModelValue(owner, resultErr)
		if resultErr != nil {
			truth, falsehood = false, false
		}
	}()
	h, err := manager.RefGet(value)
	if err != nil {
		return false, false, err
	}
	return manager.IsTrue(h), manager.IsFalse(h), nil
}
