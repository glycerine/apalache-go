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
)

type paretoModelValue struct {
	measure    big.Rat
	softCost   big.Rat
	comparable bool
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
	// C++ model::eval defaults model_completion to false in opt_context::mk_cmp.
	if err = evaluator.SetModelCompletion(false); err != nil {
		return nil, err
	}
	values = make([]paretoModelValue, len(c.objectives))
	for i := range c.objectives {
		objective := &c.objectives[i]
		if objective.soft {
			values[i].comparable = true
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
			if e == nil {
				if !matched {
					values[i].comparable = false
				} else if _, parsed := values[i].measure.SetString(text); !parsed {
					e = ast.ErrASTDomain
				} else {
					values[i].comparable = true
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

// makeParetoFormula follows pareto_base's callback order. Dominance uses
// each GE then GT comparison, followed by one flat conjunction; the persistent
// blocker invokes only LE comparisons, never constructing unused improvements.
func (c *Context) makeParetoFormula(md *model.ModelCore, dominates bool) (_ ast.ASTRef, resultErr error) {
	if md == nil {
		return ast.ASTRef{}, ast.ErrASTHandle
	}
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&refs[i]))
		}
	}()
	compare := func(index uint32, kind OptimizationModelComparison) (ast.ASTHandle, error) {
		ref, err := c.OptimizationCompareModel(index, md, kind)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		refs = append(refs, ref)
		return c.manager.RefGet(ref)
	}
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
	var weak, strict []ast.ASTHandle
	for i := range c.objectives {
		kind := OptimizationModelLE
		if dominates {
			kind = OptimizationModelGE
		}
		h, err := compare(uint32(i), kind)
		if err != nil {
			return ast.ASTRef{}, err
		}
		weak = append(weak, h)
		if dominates {
			h, err = compare(uint32(i), OptimizationModelGT)
			if err != nil {
				return ast.ASTRef{}, err
			}
			strict = append(strict, h)
		}
	}
	if dominates {
		disjunction, err := hold(c.manager.MkOrArgs(strict))
		if err != nil {
			return ast.ASTRef{}, err
		}
		weak = append(weak, disjunction)
	}
	formula, err := hold(c.manager.MkAndArgs(weak))
	if err != nil {
		return ast.ASTRef{}, err
	}
	if !dominates {
		formula, err = hold(c.manager.MkNot(formula))
		if err != nil {
			return ast.ASTRef{}, err
		}
	}
	return c.manager.NewRef(formula)
}

func (c *Context) assertParetoFormula(backend *smtcore.SMTSolver, md *model.ModelCore, dominates bool) (resultErr error) {
	if backend == nil {
		return ast.ErrASTHandle
	}
	ref, err := c.makeParetoFormula(md, dominates)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&ref)) }()
	formula, err := c.manager.RefGet(ref)
	if err != nil {
		return err
	}
	if err = c.paretoFormulaDiagnostics(formula, md, dominates); err != nil {
		return err
	}
	return backend.Assert(formula)
}

func (c *Context) closeParetoBackend() error {
	c.optimizationParetoActive = false
	if c.optimizationParetoBackend != nil {
		if err := c.optimizationParetoBackend.Close(); err != nil {
			return err
		}
		c.optimizationParetoBackend = nil
	}
	if state := c.optimizationParetoPurification; state != nil {
		if err := state.purifier.Close(); err != nil {
			return err
		}
		c.optimizationParetoPurification = nil
	}
	return nil
}

func (c *Context) newParetoBackend(assumptions []ast.ASTHandle) (backend *smtcore.SMTSolver, resultErr error) {
	backend, err := smtcore.NewOptimizationSMTSolver(c.manager, &c.params, c.logic, c.config)
	if err != nil {
		return nil, err
	}
	published := false
	ownedBackend := backend
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, ownedBackend.Close())
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
	hard := make([]ast.ASTHandle, len(c.assertions))
	for i := range c.assertions {
		hard[i] = c.assertions[i].expression
	}
	if state := c.optimizationParetoPurification; state != nil {
		hard = state.hard
	}
	for _, assertion := range hard {
		if err = backend.Assert(assertion); err != nil {
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
		if err := c.prepareParetoPurification(); err != nil {
			return nil, err
		}
		backend, err := c.newParetoBackend(assumptions)
		if err != nil {
			return nil, errors.Join(err, c.closeParetoBackend())
		}
		c.optimizationParetoBackend = backend
	}
	original := c.objectives
	c.objectives = c.optimizationParetoPurification.objectives
	defer func() { c.objectives = original }()
	c.optimizationParetoActive = true
	backend := c.optimizationParetoBackend
	status, err := backend.CheckSat(nil)
	if err != nil {
		return nil, err
	}
	unknownReason := ""
	if status == lbool.Undef {
		unknownReason, err = backend.ReasonUnknown()
		if err != nil {
			return nil, err
		}
	}
	if status != lbool.True {
		if closeErr := c.closeParetoBackend(); closeErr != nil {
			return nil, closeErr
		}
		return &opt.ArithmeticResult{Status: status, ReasonUnknown: unknownReason, Values: make([]opt.ObjectiveValue, len(c.objectives)), Optimal: status == lbool.False}, nil
	}
	canceled, cancelErr := c.manager.Canceled()
	if cancelErr != nil {
		return nil, cancelErr
	}
	if canceled {
		if closeErr := c.closeParetoBackend(); closeErr != nil {
			return nil, closeErr
		}
		return &opt.ArithmeticResult{Status: lbool.Undef, Values: make([]opt.ObjectiveValue, len(c.objectives))}, nil
	}
	var best *model.ModelCore
	defer func() {
		if resultErr != nil && best != nil {
			resultErr = errors.Join(resultErr, best.DecRef())
		}
	}()
	// PushCore clears the Go solver's published result. The C++ scoped push
	// leaves get_model available, so take an owned snapshot before pushing.
	// It represents the same model the first iteration reads in gia_pareto.
	best, err = backend.AcquireModel()
	if err != nil {
		return nil, err
	}
	if best == nil {
		return nil, ast.ErrASTHandle
	}
	labels, err := backend.Labels()
	if err != nil {
		return nil, err
	}

	if err = backend.Push(); err != nil {
		return nil, err
	}
	pushed := true
	defer func() {
		if pushed {
			resultErr = errors.Join(resultErr, backend.Pop(1))
		}
	}()
	firstIteration := true
	for status == lbool.True {
		if !firstIteration {
			canceled, cancelErr = c.manager.Canceled()
			if cancelErr != nil {
				return nil, cancelErr
			}
			if canceled {
				status = lbool.Undef
				break
			}
			candidate, acquireErr := backend.AcquireModel()
			if acquireErr != nil {
				return nil, acquireErr
			}
			if candidate == nil {
				return nil, ast.ErrASTHandle
			}
			candidateLabels, labelsErr := backend.Labels()
			if labelsErr != nil {
				return nil, errors.Join(labelsErr, candidate.DecRef())
			}

			if err = best.DecRef(); err != nil {
				return nil, errors.Join(err, candidate.DecRef())
			}
			best, labels = candidate, candidateLabels
		}
		firstIteration = false
		if err = c.paretoModelDiagnostic(best); err != nil {
			return nil, err
		}
		if err = c.assertParetoFormula(backend, best, true); err != nil {
			return nil, err
		}
		status, err = backend.CheckSat(nil)
		if err != nil {
			return nil, err
		}
		if status == lbool.Undef {
			unknownReason, err = backend.ReasonUnknown()
			if err != nil {
				return nil, err
			}
		}
	}
	if err = backend.Pop(1); err != nil {
		return nil, err
	}
	pushed = false
	if status != lbool.False {
		fixed, err := c.filterParetoModel(best)
		if err != nil {
			return nil, err
		}
		if err = best.DecRef(); err != nil {
			return nil, errors.Join(err, fixed.DecRef())
		}
		best = fixed
		if closeErr := c.closeParetoBackend(); closeErr != nil {
			return nil, closeErr
		}
		result = &opt.ArithmeticResult{Status: status, ReasonUnknown: unknownReason, Values: make([]opt.ObjectiveValue, len(c.objectives)), Model: best, Labels: labels}
		best = nil
		return result, nil
	}
	if err = c.assertParetoFormula(backend, best, false); err != nil {
		return nil, err
	}
	values, err := c.evaluateParetoModel(best)
	if err != nil {
		return nil, err
	}

	fixed, err := c.filterParetoModel(best)
	if err != nil {
		return nil, err
	}
	if err = best.DecRef(); err != nil {
		return nil, errors.Join(err, fixed.DecRef())
	}
	best = fixed
	result = &opt.ArithmeticResult{Status: lbool.True, Values: make([]opt.ObjectiveValue, len(c.objectives)), Model: best, Labels: labels, Optimal: true}
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
		if !values[i].comparable {
			result.Values[i].ArithmeticOpenBounds = true
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
