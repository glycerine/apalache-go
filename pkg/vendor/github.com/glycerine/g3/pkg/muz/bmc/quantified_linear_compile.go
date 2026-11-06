package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, qlinear::compile.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// CompileQuantifiedLinear returns the universally indexed rule implications.
// Each returned AST reference belongs to the caller.
func (c *Compiler) CompileQuantifiedLinear(rules *base.RuleSet, width uint32) (result []ast.ASTRef, resultErr error) {
	if c == nil || c.manager == nil || rules == nil {
		return nil, ast.ErrASTHandle
	}
	linear, err := IsLinear(rules)
	if err != nil || !linear {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return nil, err
	}
	q, err := newQuantifiedLinear(c.manager, width)
	if err != nil {
		return nil, err
	}
	defer func() {
		resultErr = q.disposeResults(result, resultErr)
		if resultErr != nil {
			result = nil
		}
	}()
	predicates, err := rules.GroupedPredicates()
	if err != nil {
		return nil, err
	}
	for _, predicate := range predicates {
		if err = q.compileGroup(c, rules, predicate, &result); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (q *quantifiedLinear) quantify(body ast.ASTHandle) (ast.ASTHandle, error) {
	name, err := q.manager.Symbols.Intern("T")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return q.hold(q.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{q.index}, Names: []symbol.Symbol{name}, Body: body, Weight: 1}))
}

func (q *quantifiedLinear) compileGroup(c *Compiler, rules *base.RuleSet, predicate ast.ASTHandle, result *[]ast.ASTRef) (resultErr error) {
	group, err := rules.PredicateRules(predicate)
	if err != nil {
		return err
	}
	defer func() {
		resultErr = disposeRuleOwners(q.manager, group, resultErr)
	}()
	var alternatives []ast.ASTHandle
	for i, rule := range group {
		atom, e := q.rule(predicate, uint32(i))
		if e != nil {
			return e
		}
		alternatives = append(alternatives, atom)
		body, e := q.compileRule(c, rule, predicate, uint32(i))
		if e != nil {
			return e
		}
		implication, e := q.hold(q.manager.MkImplies(atom, body))
		if e != nil {
			return e
		}
		quantified, e := q.quantify(implication)
		if e != nil {
			return e
		}
		ref, e := q.manager.NewRef(quantified)
		if e != nil {
			return e
		}
		*result = append(*result, ref)
	}
	disjunction, err := c.simplifyBoolean(false, alternatives)
	if err != nil {
		return err
	}
	q.refs = append(q.refs, disjunction)
	value, err := q.manager.RefGet(disjunction)
	if err != nil {
		return err
	}
	head, err := q.predicate(predicate, q.variable)
	if err != nil {
		return err
	}
	implication, err := q.hold(q.manager.MkImplies(head, value))
	if err != nil {
		return err
	}
	quantified, err := q.quantify(implication)
	if err != nil {
		return err
	}
	ref, err := q.manager.NewRef(quantified)
	if err != nil {
		return err
	}
	*result = append(*result, ref)
	return nil
}

func (q *quantifiedLinear) compileRule(c *Compiler, rule *base.Rule, predicate ast.ASTHandle, ruleIndex uint32) (_ ast.ASTHandle, resultErr error) {
	bindings, err := q.bindings(rule, predicate, ruleIndex)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sub, err := rewriter.NewVarSubstitution(q.manager, false)
	if err != nil {
		if sub != nil {
			err = errors.Join(err, sub.Close())
		}
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, sub.Close()) }()
	substitute := func(expr ast.ASTHandle) (ast.ASTHandle, error) {
		ref, e := q.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			return ast.ASTHandle{}, e
		}
		q.refs = append(q.refs, ref)
		if e = sub.Substitute(expr, bindings, ref); e != nil {
			return ast.ASTHandle{}, e
		}
		return q.manager.RefGet(ref)
	}
	var conditions []ast.ASTHandle
	addEqualities := func(app, decl ast.ASTHandle, current bool) error {
		arity, e := q.manager.GetArity(decl)
		if e != nil {
			return e
		}
		for i := uint32(0); i < arity; i++ {
			arg, e := q.manager.GetArg(app, i)
			if e != nil {
				return e
			}
			term, e := substitute(arg)
			if e != nil {
				return e
			}
			indexed, e := q.argument(decl, i, current)
			if e != nil {
				return e
			}
			eq, e := q.hold(q.manager.MkEq(term, indexed))
			if e != nil {
				return e
			}
			conditions = append(conditions, eq)
		}
		return nil
	}
	head, err := rule.Head()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = addEqualities(head, predicate, true); err != nil {
		return ast.ASTHandle{}, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for i := 0; i < uninterpreted; i++ {
		tail, _, e := rule.Tail(i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		decl, e := rule.TailDeclaration(i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if e = addEqualities(tail, decl, false); e != nil {
			return ast.ASTHandle{}, e
		}
		previous, e := q.hold(q.manager.MkBitVectorSub(q.variable, q.one))
		if e != nil {
			return ast.ASTHandle{}, e
		}
		pred, e := q.predicate(decl, previous)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		conditions = append(conditions, pred)
	}
	total, err := rule.TailSize()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for i := uninterpreted; i < total; i++ {
		tail, _, e := rule.Tail(i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		term, e := substitute(tail)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		conditions = append(conditions, term)
	}
	if uninterpreted != 0 {
		bound, e := q.hold(q.manager.MkBitVectorULE(q.one, q.variable))
		if e != nil {
			return ast.ASTHandle{}, e
		}
		conditions = append(conditions, bound)
	}
	ref, err := c.simplifyBoolean(true, conditions)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	q.refs = append(q.refs, ref)
	return q.manager.RefGet(ref)
}
