package spacer

// Source: src/muz/spacer/spacer_util.cpp simplify_bounds_new.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	tacticarith "github.com/glycerine/g3/pkg/tactic/arith"
	tacticcore "github.com/glycerine/g3/pkg/tactic/core"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type simplifyBoundsCleanupFailure struct {
	manager           *ast.ASTManager
	values            []ast.ASTRef
	cause, cleanupErr error
	reported          error
	reportText        string
}

func (f *simplifyBoundsCleanupFailure) Error() string {
	return f.reportText
}
func (f *simplifyBoundsCleanupFailure) Unwrap() error {
	return f.reported
}
func (f *simplifyBoundsCleanupFailure) Cleanup() error {
	f.cleanupErr = closeSimplifyBoundRefs(f.manager, f.values)
	return f.cleanupErr
}
func (f *simplifyBoundsCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.manager.NumeralService().RuntimeOwner()
}
func (f *simplifyBoundsCleanupFailure) OriginalCause() error { return f.cause }
func (f *simplifyBoundsCleanupFailure) CleanupCause() error  { return f.cleanupErr }

func closeSimplifyBoundRefs(manager *ast.ASTManager, values []ast.ASTRef) (result error) {
	for i := len(values) - 1; i >= 0; i-- {
		if values[i] == (ast.ASTRef{}) {
			continue
		}
		if err := manager.DelRef(&values[i]); err != nil {
			return errors.Join(result, err)
		}
	}
	return result
}

// simplifyBounds consumes values and returns caller-owned references from the
// exact source propagate-values/propagate-inequalities pipeline.
func simplifyBounds(manager *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, values []ast.ASTRef) (_ []ast.ASTRef, resultErr error) {
	committed := false
	defer func() {
		if committed {
			return
		}
		if cleanup := closeSimplifyBoundRefs(manager, values); cleanup != nil {
			reported := errors.Join(resultErr, cleanup)
			resultErr = &simplifyBoundsCleanupFailure{
				manager: manager, values: values, cause: resultErr, cleanupErr: cleanup,
				reported: reported, reportText: reported.Error(),
			}
		}
	}()
	goal, err := tactic.NewGoal(manager, false, false, false)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, goal.Close()) }()
	for i := range values {
		handle, err := manager.RefGet(values[i])
		if err != nil {
			return nil, err
		}
		if err = goal.Assert(handle); err != nil {
			return nil, err
		}
	}
	propagateValues, err := tacticcore.NewPropagateValuesTactic(manager, context, globals, nil)
	if err != nil {
		return nil, err
	}
	propagateInequalities, err := tacticarith.NewPropagateIneqsTactic(manager, context, nil)
	if err != nil {
		return nil, errors.Join(err, propagateValues.Close())
	}
	sequence, err := tactic.NewAndThenTactic(propagateValues, propagateInequalities)
	if err != nil {
		return nil, errors.Join(err, propagateInequalities.Close(), propagateValues.Close())
	}
	if err = sequence.IncRef(); err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, sequence.DecRef()) }()
	output := &tactic.TacticResult{}
	defer func() { resultErr = errors.Join(resultErr, output.Close()) }()
	if err = sequence.Apply(goal, output); err != nil {
		return nil, err
	}
	if len(output.Goals) != 1 {
		return nil, ast.ErrASTDomain
	}
	formulas, err := output.Goals[0].AppendFormulas(nil)
	if err != nil {
		return nil, err
	}
	result, err := refsFromHandles(manager, formulas)
	if err != nil {
		return nil, err
	}
	for i := range values {
		if err = manager.DelRef(&values[i]); err != nil {
			for j := range result {
				resultErr = errors.Join(resultErr, manager.DelRef(&result[j]))
			}
			return nil, err
		}
	}
	committed = true
	return result, nil
}
