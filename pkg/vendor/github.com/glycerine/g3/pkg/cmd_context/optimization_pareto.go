package cmd_context

// Source: src/opt/opt_pareto.cpp GIA dominance loop and
// src/opt/opt_context.cpp Pareto comparison/model callbacks.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type paretoModelValue struct {
	measure  big.Rat
	softCost big.Rat
}

func (c *Context) evaluateParetoModel(md *model.ModelCore) (values []paretoModelValue, resultErr error) {
	if md == nil {
		return nil, ast.ErrASTHandle
	}
	evaluator, err := model.NewModelEvaluator(md, c.config.ModelEvaluator)
	if err != nil {
		if evaluator != nil {
			err = errors.Join(err, evaluator.Close())
		}
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	if err = evaluator.SetModelCompletion(true); err != nil {
		return nil, err
	}
	values = make([]paretoModelValue, len(c.objectives))
	for i := range c.objectives {
		objective := &c.objectives[i]
		if objective.soft {
			for j := range objective.softTerms {
				term := &objective.softTerms[j]
				ref, e := evaluator.EvaluateValue(term.expression)
				if e != nil {
					return nil, e
				}
				resolved, e := c.manager.RefGet(ref)
				truth := e == nil && c.manager.IsTrue(resolved)
				e = errors.Join(e, c.manager.DelRef(&ref))
				if e != nil {
					return nil, e
				}
				weight := new(big.Rat).Set(&term.weight)
				if weight.Sign() < 0 {
					values[i].softCost.Add(&values[i].softCost, weight)
					weight.Neg(weight)
					truth = !truth
				}
				if truth {
					values[i].measure.Add(&values[i].measure, weight)
				} else {
					values[i].softCost.Add(&values[i].softCost, weight)
				}
			}
			continue
		}
		ref, e := evaluator.EvaluateValue(c.objectives[i].expression)
		if e != nil {
			return nil, e
		}
		h, e := c.manager.RefGet(ref)
		if e == nil {
			var text string
			var matched bool
			if c.objectives[i].bitVector {
				text, matched, e = c.manager.BitVectorNumeralText(h)
			} else {
				text, _, matched, e = c.manager.ArithmeticNumeralText(h)
			}
			if e == nil && !matched {
				e = ast.ErrASTDomain
			}
			if e == nil {
				if _, matched = values[i].measure.SetString(text); !matched {
					e = ast.ErrASTDomain
				}
			}
		}
		e = errors.Join(e, c.manager.DelRef(&ref))
		if e != nil {
			return nil, e
		}
	}
	return values, nil
}

func (c *Context) paretoSoftComparison(objective *optimizationObjective, bound *big.Rat, ge bool, hold func(ast.ASTHandle, error) (ast.ASTHandle, error)) (result ast.ASTHandle, resultErr error) {
	if objective == nil || !objective.soft || bound == nil || hold == nil {
		return result, ast.ErrASTHandle
	}
	service := c.manager.NumeralService()
	if service == nil {
		return result, ast.ErrASTHandle
	}
	coefficients := make([]unumeral.OwnedRational, 0, len(objective.softTerms))
	arguments := make([]ast.ASTHandle, 0, len(objective.softTerms))
	var threshold unumeral.OwnedRational
	defer func() {
		if threshold != (unumeral.OwnedRational{}) {
			resultErr = errors.Join(resultErr, service.Del(&threshold))
		}
		for i := len(coefficients) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, service.Del(&coefficients[i]))
		}
	}()
	for i := range objective.softTerms {
		term := &objective.softTerms[i]
		if term.weight.Sign() == 0 {
			continue
		}
		weight := new(big.Rat).Set(&term.weight)
		argument := term.expression
		if weight.Sign() < 0 {
			weight.Neg(weight)
			var err error
			argument, err = hold(c.manager.MkNot(argument))
			if err != nil {
				return result, err
			}
		}
		coefficient, err := service.NewValue()
		if err != nil {
			return result, err
		}
		coefficients = append(coefficients, coefficient)
		if err = service.SetString(&coefficients[len(coefficients)-1], weight.RatString()); err != nil {
			return result, err
		}
		arguments = append(arguments, argument)
	}
	var err error
	threshold, err = service.NewValue()
	if err != nil {
		return result, err
	}
	if err = service.SetString(&threshold, bound.RatString()); err != nil {
		return result, err
	}
	if ge {
		return c.manager.MkPseudoBooleanGE(coefficients, arguments, &threshold)
	}
	return c.manager.MkPseudoBooleanLE(coefficients, arguments, &threshold)
}

func (c *Context) paretoBitVectorNumeral(value *big.Rat, width uint32) (result ast.ASTHandle, resultErr error) {
	if value == nil || !value.IsInt() || value.Sign() < 0 || width == 0 {
		return result, ast.ErrASTDomain
	}
	service := c.manager.NumeralService()
	if service == nil {
		return result, ast.ErrASTHandle
	}
	numeral, err := service.NewValue()
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, service.Del(&numeral)) }()
	if err = service.SetString(&numeral, value.RatString()); err != nil {
		return result, err
	}
	return c.manager.BitVectorNumeral(&numeral, width)
}

func (c *Context) assertParetoFormula(backend *smtcore.SMTSolver, values []paretoModelValue, dominates bool) (resultErr error) {
	if backend == nil || len(values) != len(c.objectives) {
		return ast.ErrASTHandle
	}
	var refs []ast.ASTRef
	hold := func(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := c.manager.NewRef(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		refs = append(refs, ref)
		return h, nil
	}
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&refs[i]))
		}
	}()
	weak := make([]ast.ASTHandle, len(c.objectives))
	strict := make([]ast.ASTHandle, len(c.objectives))
	notBetter := make([]ast.ASTHandle, len(c.objectives))
	for i := range c.objectives {
		objective := &c.objectives[i]
		var numeral ast.ASTHandle
		var err error
		if objective.soft {
			weak[i], err = hold(c.paretoSoftComparison(objective, &values[i].measure, true, hold))
			if err == nil {
				notBetter[i], err = hold(c.paretoSoftComparison(objective, &values[i].measure, false, hold))
			}
		} else if objective.bitVector {
			numeral, err = hold(c.paretoBitVectorNumeral(&values[i].measure, objective.bitWidth))
		} else {
			sort, e := c.manager.GetSort(objective.expression)
			if e != nil {
				return e
			}
			kind, e := c.manager.GetDeclKind(sort)
			if e != nil {
				return e
			}
			if kind == 1 {
				numeral, err = hold(c.manager.ArithmeticIntegerNumeralText(values[i].measure.RatString()))
			} else {
				numeral, err = hold(c.manager.ArithmeticRealNumeralText(values[i].measure.RatString()))
			}
		}
		if err != nil {
			return err
		}
		if objective.soft {
			strict[i], err = hold(c.manager.MkNot(notBetter[i]))
			if err != nil {
				return err
			}
			continue
		}
		term := objective.expression
		if objective.bitVector && objective.maximize {
			weak[i], err = hold(c.manager.MkBitVectorULE(numeral, term))
			if err == nil {
				notBetter[i], err = hold(c.manager.MkBitVectorULE(term, numeral))
			}
		} else if objective.bitVector {
			weak[i], err = hold(c.manager.MkBitVectorULE(term, numeral))
			if err == nil {
				notBetter[i], err = hold(c.manager.MkBitVectorULE(numeral, term))
			}
		} else if objective.maximize {
			weak[i], err = hold(c.manager.MkArithmeticGE(term, numeral))
			if err == nil {
				notBetter[i], err = hold(c.manager.MkArithmeticLE(term, numeral))
			}
		} else {
			weak[i], err = hold(c.manager.MkArithmeticLE(term, numeral))
			if err == nil {
				notBetter[i], err = hold(c.manager.MkArithmeticGE(term, numeral))
			}
		}
		if err == nil {
			strict[i], err = hold(c.manager.MkNot(notBetter[i]))
		}
		if err != nil {
			return err
		}
	}
	if !dominates {
		conjunction, err := hold(c.manager.MkAndArgs(notBetter))
		if err != nil {
			return err
		}
		formula, err := hold(c.manager.MkNot(conjunction))
		if err != nil {
			return err
		}
		return backend.Assert(formula)
	}
	conjunction, err := hold(c.manager.MkAndArgs(weak))
	if err != nil {
		return err
	}
	disjunction, err := hold(c.manager.MkOrArgs(strict))
	if err != nil {
		return err
	}
	formula, err := hold(c.manager.MkAnd(conjunction, disjunction))
	if err != nil {
		return err
	}
	return backend.Assert(formula)
}

func (c *Context) closeParetoBackend() error {
	c.optimizationParetoActive = false
	if c.optimizationParetoBackend == nil {
		return nil
	}
	err := c.optimizationParetoBackend.Close()
	if err == nil {
		c.optimizationParetoBackend = nil
	}
	return err
}

func (c *Context) newParetoBackend(assumptions []ast.ASTHandle) (backend *smtcore.SMTSolver, resultErr error) {
	backend, err := smtcore.NewSMTSolver(c.manager, &c.params, c.logic, c.config)
	if err != nil {
		return nil, err
	}
	published := false
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, backend.Close())
		}
	}()
	if err = backend.SetProduceModels(true); err != nil {
		return nil, err
	}
	for i := range c.objectives {
		if c.objectives[i].soft {
			if err = backend.EnsurePseudoBooleanTheory(); err != nil {
				return nil, err
			}
			break
		}
	}
	for _, assertion := range c.assertions {
		if err = backend.Assert(assertion.expression); err != nil {
			return nil, err
		}
	}
	for _, assumption := range assumptions {
		if err = backend.Assert(assumption); err != nil {
			return nil, err
		}
	}
	published = true
	return backend, nil
}

func (c *Context) checkOptimizePareto(assumptions []ast.ASTHandle, continuation bool) (result *opt.ArithmeticResult, resultErr error) {
	if !continuation || !c.optimizationParetoActive {
		if err := c.closeParetoBackend(); err != nil {
			return nil, err
		}
		backend, err := c.newParetoBackend(assumptions)
		if err != nil {
			return nil, err
		}
		c.optimizationParetoBackend = backend
	}
	c.optimizationParetoActive = true
	backend := c.optimizationParetoBackend
	status, err := backend.CheckSat(nil)
	if err != nil {
		return nil, err
	}
	if status != lbool.True {
		if closeErr := c.closeParetoBackend(); closeErr != nil {
			return nil, closeErr
		}
		return &opt.ArithmeticResult{Status: status, Values: make([]opt.ObjectiveValue, len(c.objectives)), Optimal: status == lbool.False}, nil
	}
	var best *model.ModelCore
	var values []paretoModelValue
	defer func() {
		if resultErr != nil && best != nil {
			resultErr = errors.Join(resultErr, best.DecRef())
		}
	}()
	candidate, err := backend.AcquireModel()
	if err != nil {
		return nil, err
	}
	if candidate == nil {
		return nil, ast.ErrASTHandle
	}
	values, err = c.evaluateParetoModel(candidate)
	if err != nil {
		return nil, errors.Join(err, candidate.DecRef())
	}
	best = candidate
	if err = backend.Push(); err != nil {
		return nil, err
	}
	pushed := true
	defer func() {
		if pushed {
			resultErr = errors.Join(resultErr, backend.Pop(1))
		}
	}()
	for status == lbool.True {
		if err = c.assertParetoFormula(backend, values, true); err != nil {
			return nil, err
		}
		status, err = backend.CheckSat(nil)
		if err != nil {
			return nil, err
		}
		if status != lbool.True {
			break
		}
		candidate, err = backend.AcquireModel()
		if err != nil {
			return nil, err
		}
		if candidate == nil {
			return nil, ast.ErrASTHandle
		}
		candidateValues, err := c.evaluateParetoModel(candidate)
		if err != nil {
			return nil, errors.Join(err, candidate.DecRef())
		}
		if best != nil {
			if err = best.DecRef(); err != nil {
				return nil, errors.Join(err, candidate.DecRef())
			}
		}
		best, values = candidate, candidateValues
	}
	if err = backend.Pop(1); err != nil {
		return nil, err
	}
	pushed = false
	if status != lbool.False {
		if closeErr := c.closeParetoBackend(); closeErr != nil {
			return nil, closeErr
		}
		result = &opt.ArithmeticResult{Status: status, Values: make([]opt.ObjectiveValue, len(c.objectives)), Model: best}
		best = nil
		return result, nil
	}
	if err = c.assertParetoFormula(backend, values, false); err != nil {
		return nil, err
	}
	result = &opt.ArithmeticResult{Status: lbool.True, Values: make([]opt.ObjectiveValue, len(c.objectives)), Model: best, Optimal: true}
	best = nil
	for i := range values {
		if c.objectives[i].soft {
			result.Values[i].SoftCost.Set(&values[i].softCost)
			result.Values[i].Soft = true
			continue
		}
		if c.objectives[i].bitVector {
			result.Values[i].SoftCost.Set(&values[i].measure)
			if c.objectives[i].maximize {
				maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(c.objectives[i].bitWidth)), big.NewInt(1))
				result.Values[i].SoftCost.Sub(new(big.Rat).SetInt(maximum), &values[i].measure)
			}
			continue
		}
		finite := simplex.RationalInfinitesimal(&values[i].measure)
		result.Values[i] = opt.ObjectiveValue{
			Arithmetic:           smtcore.SMTArithmeticObjectiveValue{Finite: finite},
			ArithmeticModel:      smtcore.SMTArithmeticObjectiveValue{Finite: finite.Clone()},
			ArithmeticModelValid: true,
		}
	}
	return result, nil
}
