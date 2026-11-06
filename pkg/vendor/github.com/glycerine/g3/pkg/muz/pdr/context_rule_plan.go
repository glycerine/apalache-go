package pdr

// Source: context::init_rules declaration allocation and rule bucketing in
// src/muz/pdr/pdr_context.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

type contextRulePlan struct {
	order      []ast.ASTHandle
	buckets    map[ast.ASTHandle][]*base.Rule
	forms      []ast.ASTHandle
	nonBoolean bool
	arithmetic contextArithmeticPlan
}

func (c *Context) planRules(set *base.RuleSet) (_ *contextRulePlan, resultErr error) {
	if set == nil {
		return nil, ast.ErrASTDomain
	}
	count, err := set.Len()
	if err != nil {
		return nil, err
	}
	plan := &contextRulePlan{buckets: make(map[ast.ASTHandle][]*base.Rule)}
	classifier, err := newContextRuleClassifier(c.manager)
	if err != nil {
		return nil, err
	}
	seen := make(map[ast.ASTHandle]bool)
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, closeRuleBuckets(plan.buckets))
		}
	}()
	for index := 0; index < count; index++ {
		rule, x := set.Rule(index)
		if x != nil {
			return nil, x
		}
		head, x := rule.HeadDeclaration()
		if x != nil {
			return nil, errors.Join(x, rule.Close())
		}
		appendDeclaration(&plan.order, seen, head)
		plan.buckets[head] = append(plan.buckets[head], rule)
		expression, x := rule.Head()
		if x != nil {
			return nil, x
		}
		plan.forms = append(plan.forms, expression)
		if x = classifier.predicate(expression); x != nil {
			return nil, x
		}
		uninterpreted, x := rule.UninterpretedTailSize()
		if x != nil {
			return nil, x
		}
		for tail := 0; tail < uninterpreted; tail++ {
			declaration, y := rule.TailDeclaration(tail)
			if y != nil {
				return nil, y
			}
			appendDeclaration(&plan.order, seen, declaration)
			application, _, y := rule.Tail(tail)
			if y != nil {
				return nil, y
			}
			if y = classifier.predicate(application); y != nil {
				return nil, y
			}
		}
		tailSize, x := rule.TailSize()
		if x != nil {
			return nil, x
		}
		for tail := uninterpreted; tail < tailSize; tail++ {
			constraint, _, y := rule.Tail(tail)
			if y != nil {
				return nil, y
			}
			plan.forms = append(plan.forms, constraint)
			if y = classifier.expression(constraint); y != nil {
				return nil, y
			}
		}
	}
	plan.nonBoolean = classifier.nonBoolean
	plan.arithmetic, err = c.planArithmetic(plan.forms, classifier.nonBoolean, classifier.hasArithmetic)
	if err != nil {
		return nil, err
	}
	return plan, nil
}

func (p *contextRulePlan) close() error {
	if p == nil || p.buckets == nil {
		return nil
	}
	err := closeRuleBuckets(p.buckets)
	p.buckets = nil
	p.forms = nil
	return err
}
