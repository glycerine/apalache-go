package base

// Source: src/muz/base/dl_rule.cpp, rule_manager::substitute.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// SubstituteRule builds a rule with its variables replaced by bindings. The
// resulting rule deliberately keeps variable indices, including gaps, so the
// caller can compose this substitution with subsequent rule unification.
func (m *RuleManager) SubstituteRule(rule *Rule, bindings []ast.ASTHandle) (_ *Rule, resultErr error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	manager, err := rule.ASTManager()
	if err != nil {
		return nil, err
	}
	if manager != m.manager {
		return nil, ast.ErrASTHandle
	}
	sub, err := rewriter.NewVarSubstitution(manager, false)
	if err != nil {
		if sub != nil {
			err = errors.Join(err, sub.Close())
		}
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, sub.Close()) }()
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, manager.DelRef(&refs[i]))
		}
	}()
	apply := func(expression ast.ASTHandle) (ast.ASTHandle, error) {
		ref, err := manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return ast.ASTHandle{}, err
		}
		refs = append(refs, ref)
		if err = sub.Substitute(expression, bindings, ref); err != nil {
			return ast.ASTHandle{}, err
		}
		return manager.RefGet(ref)
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	head, err = apply(head)
	if err != nil {
		return nil, err
	}
	count, err := rule.TailSize()
	if err != nil {
		return nil, err
	}
	tails := make([]ast.ASTHandle, count)
	negated := make([]bool, count)
	for i := range tails {
		tail, negative, err := rule.Tail(i)
		if err != nil {
			return nil, err
		}
		tails[i], err = apply(tail)
		if err != nil {
			return nil, err
		}
		negated[i] = negative
	}
	name, err := rule.NameSymbol()
	if err != nil {
		return nil, err
	}
	return m.newRuleWithHeadPolicy(head, tails, negated, name, false, false)
}
