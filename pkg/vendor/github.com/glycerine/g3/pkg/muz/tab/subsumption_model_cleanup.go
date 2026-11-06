package tab

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/memory"
)

type subsumptionModelAttempt struct {
	manager   *ast.ASTManager
	nested    memory.ConstructionFailure
	result    ast.ASTRef
	evaluator *model.ModelEvaluator
	model     *model.ModelCore
}

func (a *subsumptionModelAttempt) Close() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	if a.result != (ast.ASTRef{}) {
		if err := a.manager.DelRef(&a.result); err != nil {
			return err
		}
	}
	if a.evaluator != nil {
		if err := a.evaluator.Close(); err != nil {
			return err
		}
		a.evaluator = nil
	}
	if a.model != nil {
		consumed, err := a.model.DecRefConsumed()
		if consumed {
			a.model = nil
		}
		if err != nil {
			return err
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	return nil
}

type subsumptionModelFailure struct {
	cause, cleanupErr, exposed error
	message                    string
	attempt                    *subsumptionModelAttempt
}

func (f *subsumptionModelFailure) Error() string { return f.message }
func (f *subsumptionModelFailure) Unwrap() error { return f.exposed }
func (f *subsumptionModelFailure) Cleanup() error {
	f.cleanupErr = f.attempt.Close()
	return f.cleanupErr
}
func (f *subsumptionModelFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *subsumptionModelFailure) OriginalCause() error { return f.cause }
func (f *subsumptionModelFailure) CleanupCause() error  { return f.cleanupErr }

func finishSubsumptionModel(a *subsumptionModelAttempt, cause error) error {
	if cause != nil {
		var nested memory.ConstructionFailure
		if errors.As(cause, &nested) {
			a.nested = nested
		}
	}
	cleanup := a.Close()
	if cleanup == nil {
		return cause
	}
	if cause == nil {
		cause = cleanup
	}
	exposed := errors.Join(cause, cleanup)
	return &subsumptionModelFailure{cause: cause, cleanupErr: cleanup, exposed: exposed, message: exposed.Error(), attempt: a}
}

func cacheFalseSubsumptionLiterals(m *ast.ASTManager, smt solver.Solver, values []ast.ASTHandle, falseLiterals map[ast.ASTHandle]ast.ASTRef, config model.ModelEvaluatorConfig) (resultErr error) {
	attempt := &subsumptionModelAttempt{manager: m}
	defer func() { resultErr = finishSubsumptionModel(attempt, resultErr) }()
	md, err := smt.AcquireModel()
	if err != nil {
		return err
	}
	if md == nil {
		return nil
	}
	attempt.model = md
	ev, err := model.NewModelEvaluator(md, config)
	if err != nil {
		return err
	}
	attempt.evaluator = ev
	for _, value := range values {
		attempt.result, err = ev.EvaluateValue(value)
		if err != nil {
			return err
		}
		actual, err := m.RefGet(attempt.result)
		if err != nil {
			return err
		}
		if m.IsFalse(actual) {
			key, cause := normalizeSubsumptionLiteral(m, value)
			if cause != nil {
				return cause
			}
			if _, exists := falseLiterals[key]; !exists {
				ref, cause := m.NewRef(key)
				if cause != nil {
					return cause
				}
				falseLiterals[key] = ref
			}
		}
		if err = m.DelRef(&attempt.result); err != nil {
			return err
		}
	}
	return nil
}

var _ memory.ConstructionFailure = (*subsumptionModelFailure)(nil)
