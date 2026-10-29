package base

// Source: src/muz/base/dl_rule.cpp extract_horn, hoist_compound_predicates,
// hoist_compound, mk_negations, ensure_app, and the already-Horn fast path of
// mk_rule/mk_horn_rule.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

func (m *RuleManager) pinTemporary(temporaries *[]ast.ASTRef, handle ast.ASTHandle) error {
	reference, err := m.manager.NewRef(handle)
	if err != nil {
		return err
	}
	*temporaries = append(*temporaries, reference)
	return nil
}

func (m *RuleManager) ensureBooleanApplication(expression ast.ASTHandle, temporaries *[]ast.ASTRef) (ast.ASTHandle, error) {
	kind, err := m.manager.GetKind(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if kind == ast.ASTApp {
		return expression, nil
	}
	sort, err := m.manager.GetSort(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	boolean, err := m.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if sort != boolean {
		return ast.ASTHandle{}, fmt.Errorf("muz/base: expected Boolean rule expression")
	}
	truth, err := m.manager.MkTrue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	application, err := m.manager.MkEq(expression, truth)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = m.pinTemporary(temporaries, application); err != nil {
		return ast.ASTHandle{}, err
	}
	return application, nil
}

func (m *RuleManager) containsRegisteredPredicate(root ast.ASTHandle) (bool, error) {
	stack := []ast.ASTHandle{root}
	seen := make(map[ast.ASTHandle]bool)
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if seen[current] {
			continue
		}
		seen[current] = true
		kind, err := m.manager.GetKind(current)
		if err != nil {
			return false, err
		}
		switch kind {
		case ast.ASTApp:
			declaration, e := m.manager.GetDecl(current)
			if e != nil {
				return false, e
			}
			if _, found := m.predicates[declaration]; found {
				return true, nil
			}
			count, e := m.manager.GetNumArgs(current)
			if e != nil {
				return false, e
			}
			for i := count; i > 0; i-- {
				argument, x := m.manager.GetArg(current, i-1)
				if x != nil {
					return false, x
				}
				stack = append(stack, argument)
			}
		case ast.ASTQuantifier:
			body, e := m.manager.GetQuantifierBody(current)
			if e != nil {
				return false, e
			}
			stack = append(stack, body)
		}
	}
	return false, nil
}

func (m *RuleManager) hoistCompoundPredicate(expression ast.ASTHandle, next *uint32, body *[]ast.ASTHandle, temporaries *[]ast.ASTRef) (ast.ASTHandle, error) {
	operator, err := m.manager.GetDeclKind(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if operator == ast.ASTOpNot {
		argument, e := m.manager.GetArg(expression, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		updated, e := m.hoistCompoundPredicate(argument, next, body, temporaries)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if updated == argument {
			return expression, nil
		}
		result, e := m.manager.MkNot(updated)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if e = m.pinTemporary(temporaries, result); e != nil {
			return ast.ASTHandle{}, e
		}
		return result, nil
	}
	predicate, err := m.isPredicate(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !predicate {
		return expression, nil
	}
	count, err := m.manager.GetNumArgs(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arguments := make([]ast.ASTHandle, count)
	changed := false
	for i := uint32(0); i < count; i++ {
		argument, e := m.manager.GetArg(expression, i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		arguments[i] = argument
		kind, e := m.manager.GetKind(argument)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if kind != ast.ASTApp {
			continue
		}
		value, e := m.manager.IsValue(argument)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if value {
			continue
		}
		sort, e := m.manager.GetSort(argument)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		variable, e := m.manager.MkVar(*next, sort)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		(*next)++
		equality, e := m.manager.MkEq(variable, argument)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if e = m.pinTemporary(temporaries, equality); e != nil {
			return ast.ASTHandle{}, e
		}
		arguments[i] = variable
		*body = append(*body, equality)
		changed = true
	}
	if !changed {
		return expression, nil
	}
	declaration, err := m.manager.GetDecl(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := m.manager.MkApp(declaration, arguments)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = m.pinTemporary(temporaries, result); err != nil {
		return ast.ASTHandle{}, err
	}
	return result, nil
}

// NewRuleFromFormula converts an already-Horn formula into one owning Rule.
// General formulas that require HNF fresh-predicate definitions are rejected
// explicitly until the HNF transformer supplies those additional rules.
func (m *RuleManager) NewRuleFromFormula(formula ast.ASTHandle, name string) (_ *Rule, resultErr error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	var temporaries []ast.ASTRef
	defer func() {
		for i := len(temporaries) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.manager.DelRef(&temporaries[i]))
		}
	}()
	kind, err := m.manager.GetKind(formula)
	if err != nil {
		return nil, err
	}
	if kind == ast.ASTQuantifier {
		forall, e := m.manager.IsForall(formula)
		if e != nil {
			return nil, e
		}
		if !forall {
			return nil, fmt.Errorf("muz/base: Horn rule must be universally quantified")
		}
		formula, err = m.manager.GetQuantifierBody(formula)
		if err != nil {
			return nil, err
		}
	}
	var head ast.ASTHandle
	var body []ast.ASTHandle
	operator := int32(-1)
	kind, err = m.manager.GetKind(formula)
	if err != nil {
		return nil, err
	}
	if kind == ast.ASTApp {
		operator, err = m.manager.GetDeclKind(formula)
		if err != nil {
			return nil, err
		}
	}
	if operator == ast.ASTOpImplies {
		antecedent, e := m.manager.GetArg(formula, 0)
		if e != nil {
			return nil, e
		}
		head, e = m.manager.GetArg(formula, 1)
		if e != nil {
			return nil, e
		}
		body, err = m.manager.FlattenAnd([]ast.ASTHandle{antecedent})
		if err != nil {
			return nil, err
		}
	} else {
		head = formula
	}
	head, err = m.ensureBooleanApplication(head, &temporaries)
	if err != nil {
		return nil, err
	}
	used, err := ast.NewASTUsedVars(m.manager)
	if err != nil {
		return nil, err
	}
	if err = used.Collect(formula); err != nil {
		return nil, err
	}
	next := used.MaxFoundVarIndexPlusOne()
	head, err = m.hoistCompoundPredicate(head, &next, &body, &temporaries)
	if err != nil {
		return nil, err
	}
	originalBody := len(body)
	for i := 0; i < originalBody; i++ {
		body[i], err = m.ensureBooleanApplication(body[i], &temporaries)
		if err != nil {
			return nil, err
		}
		body[i], err = m.hoistCompoundPredicate(body[i], &next, &body, &temporaries)
		if err != nil {
			return nil, err
		}
	}
	negated := make([]bool, len(body))
	for i, expression := range body {
		operator, e := m.manager.GetDeclKind(expression)
		if e != nil {
			return nil, e
		}
		if operator == ast.ASTOpNot {
			argument, x := m.manager.GetArg(expression, 0)
			if x != nil {
				return nil, x
			}
			predicate, x := m.isPredicate(argument)
			if x != nil {
				return nil, x
			}
			if predicate {
				body[i], negated[i] = argument, true
				continue
			}
		}
		predicate, e := m.isPredicate(expression)
		if e != nil {
			return nil, e
		}
		if !predicate {
			contains, x := m.containsRegisteredPredicate(expression)
			if x != nil {
				return nil, x
			}
			if contains {
				return nil, fmt.Errorf("muz/base: formula requires Horn normal-form definitions")
			}
		}
	}
	return m.NewRule(head, body, negated, name)
}
