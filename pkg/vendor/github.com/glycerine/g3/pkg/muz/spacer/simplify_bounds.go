package spacer

// Source: src/muz/spacer/spacer_util.cpp simplify_bounds_new.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	tacticarith "github.com/glycerine/g3/pkg/tactic/arith"
	tacticcore "github.com/glycerine/g3/pkg/tactic/core"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// simplifyBounds consumes values and returns caller-owned references from the
// exact source propagate-values/propagate-inequalities pipeline.
func simplifyBounds(manager *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, values []ast.ASTRef) (_ []ast.ASTRef, resultErr error) {
	committed := false
	defer func() {
		if committed {
			return
		}
		for i := range values {
			if values[i] != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, manager.DelRef(&values[i]))
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
