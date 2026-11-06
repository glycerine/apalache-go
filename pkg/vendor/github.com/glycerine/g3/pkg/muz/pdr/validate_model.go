package pdr

// Source: pdr_context.cpp context::validate_model and get_level_property.
// Validation intentionally uses live normalized rules, no model converter,
// no completion, and no independent query-falsity assertion.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (c *Context) validateUNSATResult() (resultErr error) {
	if err := c.open(); err != nil {
		return err
	}
	md, err := model.NewModel(c.manager)
	if err != nil {
		return err
	}
	owner := &pdrModelExportOwner{manager: c.manager, model: md}
	attempt := newPDRValidationAttempt(c.manager)
	attempt.modelOwners = append(attempt.modelOwners, owner)
	defer func() { resultErr = finishPDRValidation(attempt, resultErr) }()
	pin := func(h ast.ASTHandle) error {
		r, err := c.manager.NewRef(h)
		if err == nil {
			owner.references = append(owner.references, r)
		}
		return err
	}
	abstractor, err := ast.NewExprAbstractor(c.manager)
	if err != nil {
		return err
	}
	attempt.abstractors = append(attempt.abstractors, abstractor)
	for _, declaration := range c.order {
		if declaration == c.queryDecl {
			continue
		}
		pt := c.transformers[declaration]
		bodyRef, err := pt.FormulasRef(c.inductive, false)
		if err != nil {
			return err
		}
		owner.references = append(owner.references, bodyRef)
		body, err := c.manager.RefGet(bodyRef)
		if err != nil {
			return err
		}
		body, err = c.owner.FormulaNewToOld(body, 0, false)
		if err != nil {
			return err
		}
		if err = pin(body); err != nil {
			return err
		}
		fixed, err := c.fixupInductiveClausesRef(body)
		if fixed != (ast.ASTRef{}) {
			owner.references = append(owner.references, fixed)
		}
		if err != nil {
			return err
		}
		body, err = c.manager.RefGet(fixed)
		if err != nil {
			return err
		}
		arguments := make([]ast.ASTHandle, len(pt.signature))
		for i := range arguments {
			arguments[i], err = c.manager.MkApp(pt.signature[len(arguments)-i-1], nil)
			if err != nil {
				return err
			}
			if err = pin(arguments[i]); err != nil {
				return err
			}
		}
		abstracted, err := c.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		owner.references = append(owner.references, abstracted)
		if err = abstractor.Abstract(0, arguments, body, abstracted); err != nil {
			return err
		}
		body, err = c.manager.RefGet(abstracted)
		if err != nil {
			return err
		}
		if len(arguments) == 0 {
			if err = md.Core().RegisterConstant(declaration, body); err != nil {
				return err
			}
		} else {
			fi, err := model.NewFunctionInterpretation(c.manager, uint32(len(arguments)))
			if err != nil {
				return err
			}
			owner.interpretations = append(owner.interpretations, fi)
			if err = fi.SetElse(body); err != nil {
				return err
			}
			if err = md.Core().RegisterFunction(declaration, fi); err != nil {
				return err
			}
			owner.interpretations = owner.interpretations[:len(owner.interpretations)-1]
		}
	}
	if err = c.traceInductiveModel(md); err != nil {
		return err
	}
	return c.validateInductiveModel(md)
}

func (c *Context) validateInductiveModel(md *model.Model) (resultErr error) {
	if err := c.open(); err != nil {
		return err
	}
	if md == nil || md.Core().Manager() != c.manager {
		return ast.ErrASTHandle
	}
	evaluator, err := model.NewModelEvaluator(md.Core(), c.owner.contexts.config.ModelEvaluator)
	if err != nil {
		return err
	}
	attempt := newPDRValidationAttempt(c.manager)
	attempt.evaluators = append(attempt.evaluators, evaluator)
	defer func() { resultErr = finishPDRValidation(attempt, resultErr) }()
	if err = evaluator.SetModelCompletion(false); err != nil {
		return err
	}
	for _, declaration := range c.order {
		rules, err := c.transformers[declaration].Rules()
		if err != nil {
			return err
		}
		attempt.rules = append(attempt.rules, rules...)
		for _, rule := range rules {
			if err := c.validateModelRule(evaluator, rule); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Context) validateModelRule(evaluator *model.ModelEvaluator, rule *base.Rule) (resultErr error) {
	attempt := newPDRValidationAttempt(c.manager)
	defer func() { resultErr = finishPDRValidation(attempt, resultErr) }()
	keep := func(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		r, err := c.manager.NewRef(h)
		if err == nil {
			attempt.references = append(attempt.references, r)
		}
		return h, err
	}
	eval := func(h ast.ASTHandle) (ast.ASTHandle, error) {
		r, err := evaluator.EvaluateValue(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		attempt.references = append(attempt.references, r)
		return c.manager.RefGet(r)
	}
	head, err := rule.Head()
	if err != nil {
		return err
	}
	head, err = eval(head)
	if err != nil {
		return err
	}
	negated, err := keep(c.manager.MkNot(head))
	if err != nil {
		return err
	}
	formulas := []ast.ASTHandle{negated}
	uts, err := rule.UninterpretedTailSize()
	if err != nil {
		return err
	}
	ts, err := rule.TailSize()
	if err != nil {
		return err
	}
	for i := 0; i < ts; i++ {
		tail, _, err := rule.Tail(i)
		if err != nil {
			return err
		}
		if i < uts {
			tail, err = eval(tail)
			if err != nil {
				return err
			}
		}
		formulas = append(formulas, tail)
	}
	counterexample, err := keep(c.manager.MkAndArray(formulas))
	if err != nil {
		return err
	}
	free, err := rewriter.NewExprFreeVars(c.manager)
	if err != nil {
		return err
	}
	if err = free.Collect(counterexample); err != nil {
		return err
	}
	boolean, err := c.manager.MkBoolSort()
	if err != nil {
		return err
	}
	if err = free.SetDefaultSort(boolean); err != nil {
		return err
	}
	names := make([]symbol.Symbol, free.Size())
	for i := range names {
		names[i] = symbol.Numerical(uint32(i))
	}
	if err = free.Reverse(); err != nil {
		return err
	}
	if !free.Empty() {
		counterexample, err = keep(c.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: free.Sorts(), Names: names, Body: counterexample}))
		if err != nil {
			return err
		}
	}
	solver, err := smtcore.NewSMTSolver(c.manager, c.owner.contexts.params, "", c.owner.contexts.config)
	if err != nil {
		return err
	}
	attempt.solvers = append(attempt.solvers, solver)
	if err = solver.Assert(counterexample); err != nil {
		return err
	}
	status, err := solver.CheckSat(nil)
	if err != nil {
		return err
	}
	if status != lbool.False {
		return c.modelValidationError(counterexample)
	}
	return nil
}

func (c *Context) modelValidationError(formula ast.ASTHandle) (resultErr error) {
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: c.manager.Budget()})
	if err != nil {
		return err
	}
	attempt := newPDRValidationAttempt(c.manager)
	attempt.streams = append(attempt.streams, out)
	defer func() { resultErr = finishPDRValidation(attempt, resultErr) }()
	config := c.owner.contexts.config
	tree, err := c.manager.SMT2PPExprTreeWithParams(formula, nil, config.Params, config.Globals, c.owner.contexts.params)
	if err != nil {
		return err
	}
	formats, err := c.manager.GetFormatManager()
	if err != nil {
		return err
	}
	attempt.manager = formats
	ref, err := formats.NewRef(tree)
	if err != nil {
		return err
	}
	attempt.references = append(attempt.references, ref)
	local := config.Params.EmptyValue()
	if err = c.manager.PrintFormat(out, tree, config.Params, &local, config.Globals); err != nil {
		return err
	}
	text, err := out.String()
	if err != nil {
		return err
	}
	return fmt.Errorf("rule validation failed when checking: %s", text)
}
