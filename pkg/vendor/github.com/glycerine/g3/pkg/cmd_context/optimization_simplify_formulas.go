package cmd_context

// Source: src/opt/opt_context.cpp normalize/simplify_fmls.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/arith"
	"github.com/glycerine/g3/pkg/tactic/bv"
	tacticcore "github.com/glycerine/g3/pkg/tactic/core"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type optimizationSimplifiedFormulas struct {
	decoded      opt.DecodedFormulaSet
	refs         []ast.ASTRef
	model        tactic.ModelConverter
	manager      *ast.ASTManager
	inconsistent bool
	closed       bool
}

func (f *optimizationSimplifiedFormulas) Close() (result error) {
	if f == nil || f.manager == nil {
		return ast.ErrASTHandle
	}
	if f.closed {
		return nil
	}
	for len(f.refs) != 0 {
		i := len(f.refs) - 1
		result = errors.Join(result, f.manager.DelRef(&f.refs[i]))
		f.refs = f.refs[:i]
	}
	if f.model != nil {
		consumed, err := f.model.DecRefConsumed()
		result = errors.Join(result, err)
		if !consumed {
			return errors.Join(result, tactic.ErrModelConverterOwner)
		}
		f.model = nil
	}
	if result == nil {
		f.decoded = opt.DecodedFormulaSet{}
		f.closed = true
	}
	return result
}

func (c *Context) simplifyOptimizationFormulas(set *opt.ObjectiveFormulaSet) (result *optimizationSimplifiedFormulas, resultErr error) {
	if c == nil || c.manager == nil || set == nil {
		return nil, ast.ErrASTHandle
	}
	result = &optimizationSimplifiedFormulas{manager: c.manager}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	formulas, err := set.Handles()
	if err != nil {
		return nil, err
	}
	goal, err := tactic.NewGoal(c.manager, false, true, false)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, goal.Close()) }()
	for _, formula := range formulas {
		if err = goal.Assert(formula); err != nil {
			return nil, err
		}
	}
	var owned []tactic.Tactic
	defer func() {
		for i := len(owned) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, owned[i].DecRef())
		}
	}()
	keep := func(value tactic.Tactic, cause error) (tactic.Tactic, error) {
		if cause != nil {
			return nil, cause
		}
		if cause = value.IncRef(); cause != nil {
			return nil, cause
		}
		owned = append(owned, value)
		return value, nil
	}
	var stages []tactic.Tactic
	appendStage := func(value tactic.Tactic, cause error) error {
		value, cause = keep(value, cause)
		if cause == nil {
			stages = append(stages, value)
		}
		return cause
	}
	m, params, globals := c.manager, c.config.Params, c.config.Globals
	if err = appendStage(tacticcore.MakeSimplifyTactic(m, params, globals, &c.params)); err == nil {
		var propagate *tacticcore.PropagateValuesTactic
		propagate, err = tacticcore.NewPropagateValuesTactic(m, params, globals, &c.params)
		if err == nil {
			child := propagate
			var clean tactic.Tactic
			clean, err = tactic.NewCleanTactic(child)
			if err != nil {
				err = errors.Join(err, child.Close())
			} else {
				err = appendStage(clean, nil)
			}
		}
	}
	if err == nil {
		err = appendStage(tacticcore.MakeSolveEquationsTactic(m, params, globals, &c.params, c.config.ModelEvaluator))
	}
	if err == nil {
		err = appendStage(tacticcore.MakeSimplifyTactic(m, params, globals, &c.params))
	}
	optParams, paramsErr := opt.NewParams(params, globals, &c.params)
	err = errors.Join(err, paramsErr)
	if err == nil {
		var enabled bool
		enabled, err = optParams.Elim01()
		if err == nil && enabled {
			var datatype *bv.DT2BVTactic
			datatype, err = bv.NewDT2BVTactic(m, params, nil, c.config.ModelEvaluator)
			if err == nil {
				var clean tactic.Tactic
				clean, err = tactic.NewCleanTactic(datatype)
				if err == nil {
					err = appendStage(clean, nil)
				}
				if err != nil && clean == nil {
					err = errors.Join(err, datatype.Close())
				}
			}
			if err == nil {
				err = appendStage(arith.MakeElim01Tactic(m, params, globals, nil, c.config.ModelEvaluator))
			}
			var compileEquality bool
			if err == nil {
				compileEquality, err = optParams.PBCompileEquality()
			}
			if err == nil {
				var local uparams.ParamsRef
				local, err = params.NewValue()
				if err == nil {
					key := "compile_equality"
					err = params.SetBoolName(&local, &key, compileEquality)
					if err == nil {
						err = appendStage(arith.MakeLIA2CardTactic(m, params, globals, &local))
					}
					err = errors.Join(err, params.Del(&local))
				}
			}
			if err == nil {
				err = appendStage(arith.MakeEq2BVTactic(m, params, globals))
			}
			if err == nil {
				err = appendStage(tacticcore.MakeSimplifyTactic(m, params, globals, &c.params))
			}
		}
	}
	if optParams != nil {
		err = errors.Join(err, optParams.Close())
	}
	if err != nil {
		return nil, err
	}
	sequence, err := tactic.NewAndThenTactic(stages...)
	if err != nil {
		return nil, err
	}
	if err = sequence.IncRef(); err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, sequence.DecRef()) }()
	var applied tactic.TacticResult
	defer func() { resultErr = errors.Join(resultErr, applied.Close()) }()
	if err = sequence.Apply(goal, &applied); err != nil {
		return nil, err
	}
	if len(applied.Goals) != 1 {
		return nil, ast.ErrASTDomain
	}
	output := applied.Goals[0]
	inconsistent, err := output.Inconsistent()
	if err != nil {
		return nil, err
	}
	if inconsistent {
		result.inconsistent = true
		return result, nil
	}
	count, err := output.Size()
	if err != nil {
		return nil, err
	}
	formulas = make([]ast.ASTHandle, count)
	for i := range formulas {
		formulas[i], err = output.Form(uint32(i))
		if err != nil {
			return nil, err
		}
		ref, e := m.NewRef(formulas[i])
		if e != nil {
			return nil, e
		}
		result.refs = append(result.refs, ref)
	}
	result.decoded, err = set.DecodeHandles(formulas)
	if err != nil {
		return nil, err
	}
	if applied.ModelConverter != nil {
		if err = applied.ModelConverter.IncRef(); err != nil {
			return nil, err
		}
		result.model = applied.ModelConverter
	}
	return result, nil
}
