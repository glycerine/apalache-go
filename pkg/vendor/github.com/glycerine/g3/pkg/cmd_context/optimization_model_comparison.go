package cmd_context

// Source: src/opt/opt_context.cpp:472-545, mk_le/mk_ge/mk_gt/mk_cmp.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type OptimizationModelComparison uint8

const (
	OptimizationModelLE OptimizationModelComparison = iota
	OptimizationModelGE
	OptimizationModelGT
)

// OptimizationCompareModel returns the source objective/model comparison as
// one owned Boolean AST reference. GE/LE follow objective preference order,
// so minimization reverses the underlying arithmetic comparison. GT is the
// negation of LE, matching context::mk_gt.
func (c *Context) OptimizationCompareModel(index uint32, md *model.ModelCore, comparison OptimizationModelComparison) (_ ast.ASTRef, resultErr error) {
	if err := c.valid(); err != nil {
		return ast.ASTRef{}, err
	}
	if uint64(index) >= uint64(len(c.objectives)) {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	objective := &c.objectives[index]
	if md == nil {
		return ast.ASTRef{}, ast.ErrASTHandle
	}
	if comparison > OptimizationModelGT {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	isGE := comparison == OptimizationModelGE
	if comparison == OptimizationModelGT {
		isGE = false
	}
	var roots []ast.ASTRef
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&roots[i]))
		}
	}()
	hold := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := c.manager.NewRef(value)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		roots = append(roots, ref)
		return value, nil
	}
	var formula ast.ASTHandle
	var err error
	if objective.soft {
		formula, err = c.optimizationSoftModelComparison(objective, md, isGE, hold)
	} else {
		formula, err = c.optimizationTermModelComparison(objective, md, isGE, hold)
	}
	if err != nil {
		return ast.ASTRef{}, err
	}
	if comparison == OptimizationModelGT {
		formula, err = hold(c.manager.MkNotSimplified(formula))
		if err != nil {
			return ast.ASTRef{}, err
		}
	}
	return c.manager.NewRef(formula)
}

func (c *Context) optimizationTermModelComparison(objective *optimizationObjective, md *model.ModelCore, isGE bool, hold func(ast.ASTHandle, error) (ast.ASTHandle, error)) (result ast.ASTHandle, resultErr error) {
	if objective == nil || objective.soft {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	if !objective.maximize {
		isGE = !isGE
	}
	ref, evaluated, err := c.evaluateOptimizationModel(md, objective.expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !evaluated {
		return c.manager.MkTrue()
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&ref)) }()
	value, err := c.manager.RefGet(ref)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	matched := false
	if objective.bitVector {
		_, matched, err = c.manager.BitVectorNumeralText(value)
	} else {
		_, _, matched, err = c.manager.ArithmeticNumeralText(value)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !matched {
		return c.manager.MkTrue()
	}
	value, err = hold(value, nil)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if objective.bitVector {
		if isGE {
			return c.manager.MkBitVectorULE(value, objective.expression)
		}
		return c.manager.MkBitVectorULE(objective.expression, value)
	}
	if isGE {
		return c.manager.MkArithmeticGE(objective.expression, value)
	}
	return c.manager.MkArithmeticGE(value, objective.expression)
}

func (c *Context) optimizationSoftModelComparison(objective *optimizationObjective, md *model.ModelCore, isGE bool, hold func(ast.ASTHandle, error) (ast.ASTHandle, error)) (result ast.ASTHandle, resultErr error) {
	if objective == nil || !objective.soft {
		return result, ast.ErrASTHandle
	}
	var err error
	service := c.manager.NumeralService()
	if service == nil {
		return result, ast.ErrASTHandle
	}
	coefficients := make([]unumeral.OwnedRational, len(objective.softTerms))
	arguments := make([]ast.ASTHandle, len(objective.softTerms))
	var threshold unumeral.OwnedRational
	defer func() {
		if threshold != (unumeral.OwnedRational{}) {
			resultErr = errors.Join(resultErr, service.Del(&threshold))
		}
		for i := len(coefficients) - 1; i >= 0; i-- {
			if coefficients[i] != (unumeral.OwnedRational{}) {
				resultErr = errors.Join(resultErr, service.Del(&coefficients[i]))
			}
		}
	}()
	var bound big.Rat
	for i := range objective.softTerms {
		term := &objective.softTerms[i]
		arguments[i] = term.expression
		coefficients[i], err = service.NewValue()
		if err != nil {
			return result, err
		}
		if err = service.SetString(&coefficients[i], term.weight.RatString()); err != nil {
			return result, err
		}
		ref, evaluated, err := c.evaluateOptimizationModel(md, term.expression)
		if err != nil {
			return result, err
		}
		if !evaluated {
			continue
		}
		value, err := c.manager.RefGet(ref)
		truth := err == nil && c.manager.IsTrue(value)
		err = errors.Join(err, c.manager.DelRef(&ref))
		if err != nil {
			return result, err
		}
		if truth {
			bound.Add(&bound, &term.weight)
		}
	}
	threshold, err = service.NewValue()
	if err != nil {
		return result, err
	}
	if err = service.SetString(&threshold, bound.RatString()); err != nil {
		return result, err
	}
	if isGE {
		return hold(c.manager.MkPseudoBooleanGE(coefficients, arguments, &threshold))
	}
	return hold(c.manager.MkPseudoBooleanLE(coefficients, arguments, &threshold))
}

// Source model::eval constructs an evaluator per call and catches only
// model_evaluator_exception (the rewriter_exception typedef). Allocation and
// owner failures propagate; evaluation failure leaves the comparison neutral.
func (c *Context) evaluateOptimizationModel(md *model.ModelCore, expression ast.ASTHandle) (ref ast.ASTRef, evaluated bool, resultErr error) {
	defer func() {
		if resultErr != nil && ref != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&ref))
			ref = ast.ASTRef{}
			evaluated = false
		}
	}()

	evaluator, err := model.NewModelEvaluator(md, c.config.ModelEvaluator)
	if evaluator != nil {
		defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	}
	if err != nil {
		return ast.ASTRef{}, false, err
	}
	if err = evaluator.SetModelCompletion(false); err != nil {
		return ast.ASTRef{}, false, err
	}
	ref, err = evaluator.EvaluateValue(expression)
	if rewriter.IsRewriterException(err) {
		return ast.ASTRef{}, false, nil
	}
	return ref, err == nil, err
}

func (c *Context) OptimizationModelLE(index uint32, md *model.ModelCore) (ast.ASTRef, error) {
	return c.OptimizationCompareModel(index, md, OptimizationModelLE)
}

func (c *Context) OptimizationModelGE(index uint32, md *model.ModelCore) (ast.ASTRef, error) {
	return c.OptimizationCompareModel(index, md, OptimizationModelGE)
}

func (c *Context) OptimizationModelGT(index uint32, md *model.ModelCore) (ast.ASTRef, error) {
	return c.OptimizationCompareModel(index, md, OptimizationModelGT)
}
