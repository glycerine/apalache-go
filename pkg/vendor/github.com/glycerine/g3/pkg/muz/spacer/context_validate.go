package spacer

// Source: src/muz/spacer/spacer_context.cpp context::validate.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (c *Context) validate() error {
	if !c.validateResult {
		return nil
	}
	switch c.lastResult {
	case lbool.True:
		answer, err := c.groundSatAnswer()
		if err != nil {
			return fmt.Errorf("spacer: counterexample validation failed: %w", err)
		}
		return c.manager.DelRef(&answer)
	case lbool.False:
		return c.validateInductiveModel()
	default:
		return nil
	}
}

func (c *Context) validateInductiveModel() (resultErr error) {
	property, err := c.inductiveProperty()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, property.Close()) }()
	md, err := property.Model()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, md.Close()) }()
	evaluator, err := model.NewModelEvaluator(md.Core(), c.owner.contexts[0].config.ModelEvaluator)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	for _, declaration := range c.order {
		rules, err := c.transformers[declaration].Rules()
		if err != nil {
			return err
		}
		for index, rule := range rules {
			err = c.validateRule(evaluator, rule)
			closeErr := rule.Close()
			rules[index] = nil
			if err != nil || closeErr != nil {
				for _, pending := range rules[index+1:] {
					closeErr = errors.Join(closeErr, pending.Close())
				}
				return errors.Join(err, closeErr)
			}
		}
	}
	return nil
}

func (c *Context) validateRule(evaluator *model.ModelEvaluator, rule *base.Rule) (resultErr error) {
	head, err := rule.Head()
	if err != nil {
		return err
	}
	evaluatedHead, err := evaluator.EvaluateValue(head)
	if err != nil {
		return err
	}
	refs := []ast.ASTRef{evaluatedHead}
	defer func() { resultErr = errors.Join(resultErr, releaseNormalizeRefs(c.manager, refs)) }()
	evaluatedHeadHandle, err := c.manager.RefGet(evaluatedHead)
	if err != nil {
		return err
	}
	notHead, err := c.manager.MkNot(evaluatedHeadHandle)
	if err != nil {
		return err
	}
	conjuncts := []ast.ASTHandle{notHead}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return err
	}
	tailSize, err := rule.TailSize()
	if err != nil {
		return err
	}
	for index := 0; index < tailSize; index++ {
		tail, negated, err := rule.Tail(index)
		if err != nil {
			return err
		}
		if index < uninterpreted {
			value, err := evaluator.EvaluateValue(tail)
			if err != nil {
				return err
			}
			refs = append(refs, value)
			tail, err = c.manager.RefGet(value)
			if err != nil {
				return err
			}
		}
		if negated {
			tail, err = c.manager.MkNot(tail)
			if err != nil {
				return err
			}
		}
		conjuncts = append(conjuncts, tail)
	}
	formula, err := c.manager.MkAndArray(conjuncts)
	if err != nil {
		return err
	}
	free, err := rewriter.NewExprFreeVars(c.manager)
	if err != nil {
		return err
	}
	if err = free.Collect(formula); err != nil {
		return err
	}
	if !free.Empty() {
		boolean, err := c.manager.MkBoolSort()
		if err != nil {
			return err
		}
		if err = free.SetDefaultSort(boolean); err != nil {
			return err
		}
		if err = free.Reverse(); err != nil {
			return err
		}
		names := make([]usymbol.Symbol, free.Size())
		for index := range names {
			names[index] = usymbol.Numerical(uint32(len(names) - index - 1))
		}
		formula, err = c.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: free.Sorts(), Names: names, Body: formula})
		if err != nil {
			return err
		}
	}
	checker, err := c.owner.Fresh(2)
	if err != nil {
		return err
	}
	if err = checker.Assert(formula); err != nil {
		return errors.Join(err, checker.Close())
	}
	status, err := checker.CheckSat(nil)
	err = errors.Join(err, checker.Close())
	if err != nil {
		return err
	}
	if status != lbool.False {
		return fmt.Errorf("spacer: rule validation failed with %s", status)
	}
	return nil
}
