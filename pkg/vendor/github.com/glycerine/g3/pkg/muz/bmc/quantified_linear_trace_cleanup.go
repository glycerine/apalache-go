package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp qlinear::get_model rule tracing
// and model-evaluator/indexed-expression lifetimes.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
)

type quantifiedLinearTraceOwner struct {
	manager   *ast.ASTManager
	nested    memory.ConstructionFailure
	terms     *quantifiedLinear
	evaluator *model.ModelEvaluator
	group     []*base.Rule
	selected  *base.Rule
	steps     []quantifiedLinearStep
}

func (o *quantifiedLinearTraceOwner) closeGroup() error {
	for i := len(o.group) - 1; i >= 0; i-- {
		if o.group[i] == nil {
			continue
		}
		if err := o.group[i].Close(); err != nil {
			return err
		}
		o.group[i] = nil
	}
	o.group = nil
	return nil
}

func (o *quantifiedLinearTraceOwner) closeLocals() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	if err := o.closeGroup(); err != nil {
		return err
	}
	if o.evaluator != nil {
		if err := o.evaluator.Close(); err != nil {
			return err
		}
		o.evaluator = nil
	}
	if o.terms != nil {
		if err := o.terms.close(); err != nil {
			return err
		}
		o.terms = nil
	}
	return nil
}

func (o *quantifiedLinearTraceOwner) closeOutputs() error {
	if o.selected != nil {
		if err := o.selected.Close(); err != nil {
			return err
		}
		o.selected = nil
	}
	for i := len(o.steps) - 1; i >= 0; i-- {
		if o.steps[i].rule == nil {
			continue
		}
		if err := o.steps[i].rule.Close(); err != nil {
			return err
		}
		o.steps[i].rule = nil
	}
	o.steps = nil
	return nil
}

func (o *quantifiedLinearTraceOwner) close() error {
	if err := o.closeLocals(); err != nil {
		return err
	}
	return o.closeOutputs()
}

type quantifiedLinearTraceFailure struct {
	owner                       *quantifiedLinearTraceOwner
	cause, cleanupErr, reported error
	message                     string
}

func (f *quantifiedLinearTraceFailure) Error() string        { return f.message }
func (f *quantifiedLinearTraceFailure) Unwrap() error        { return f.reported }
func (f *quantifiedLinearTraceFailure) OriginalCause() error { return f.cause }
func (f *quantifiedLinearTraceFailure) CleanupCause() error  { return f.cleanupErr }
func (f *quantifiedLinearTraceFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *quantifiedLinearTraceFailure) Cleanup() error {
	f.cleanupErr = f.owner.close()
	return f.cleanupErr
}

func finishQuantifiedLinearTrace(o *quantifiedLinearTraceOwner, cause error) ([]quantifiedLinearStep, error) {
	if cause != nil {
		errors.As(cause, &o.nested)
	}
	cleanup := o.closeLocals()
	if cause == nil && cleanup == nil {
		steps := o.steps
		o.steps = nil
		return steps, nil
	}
	if cause == nil {
		cause = cleanup
	}
	if cleanup == nil {
		cleanup = o.closeOutputs()
	}
	if cleanup == nil {
		return nil, cause
	}
	reported := errors.Join(cause, cleanup)
	return nil, &quantifiedLinearTraceFailure{
		owner: o, cause: cause, cleanupErr: cleanup,
		reported: reported, message: reported.Error(),
	}
}

var _ memory.ConstructionFailure = (*quantifiedLinearTraceFailure)(nil)
