package rel

// Source: src/muz/rel/dl_mk_explanations.cpp, transform_rules and the
// predicate-inheritance portion of transform_facts for rule-defined predicates.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// TransformRulesOnly builds the explanation rules and output bridges for a
// rule set without transferring existing relation-manager facts. The caller
// owns the returned set. An empty source has no transformed rule set.
func (t *ExplanationTransformer) TransformRulesOnly(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if source == nil {
		return nil, ast.ErrASTHandle
	}
	manager, err := source.RuleManager()
	if err != nil {
		return nil, err
	}
	if manager != t.rules {
		return nil, ast.ErrASTHandle
	}
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, nil
	}
	result, err := base.NewRuleSet(t.rules)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	// Source transform_facts inherits each predicate with rules before the
	// separate rule pass. Preserve first-seen head order for this subset.
	seen := make(map[ast.ASTHandle]struct{})
	for i := 0; i < count; i++ {
		rule, err := source.Rule(i)
		if err != nil {
			return nil, err
		}
		declaration, getErr := rule.HeadDeclaration()
		getErr = errors.Join(getErr, rule.Close())
		if getErr != nil {
			return nil, getErr
		}
		if _, found := seen[declaration]; found {
			continue
		}
		seen[declaration] = struct{}{}
		explained, err := t.ExplanationDeclaration(declaration)
		if err != nil {
			return nil, err
		}
		if err = result.InheritPredicate(source, declaration, explained); err != nil {
			return nil, err
		}
	}
	for i := 0; i < count; i++ {
		rule, err := source.Rule(i)
		if err != nil {
			return nil, err
		}
		converted, convertErr := t.ExplanationRule(rule)
		convertErr = errors.Join(convertErr, rule.Close())
		if convertErr != nil {
			if converted != nil {
				convertErr = errors.Join(convertErr, converted.Close())
			}
			return nil, convertErr
		}
		addErr := result.Add(converted)
		addErr = errors.Join(addErr, converted.Close())
		if addErr != nil {
			return nil, addErr
		}
	}
	outputs, err := source.OutputPredicates()
	if err != nil {
		return nil, err
	}
	for _, declaration := range outputs {
		if err = t.addExplanationOutputBridge(result, declaration); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (t *ExplanationTransformer) addExplanationOutputBridge(target *base.RuleSet, declaration ast.ASTHandle) (resultErr error) {
	m := t.manager
	arity, err := m.GetArity(declaration)
	if err != nil {
		return err
	}
	arguments := make([]ast.ASTHandle, arity)
	for i := uint32(0); i < arity; i++ {
		sort, err := m.GetDomain(declaration, i)
		if err != nil {
			return err
		}
		arguments[i], err = m.MkVar(i, sort)
		if err != nil {
			return err
		}
	}
	head, err := m.MkApp(declaration, arguments)
	if err != nil {
		return err
	}
	headRef, err := m.NewRef(head)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&headRef)) }()
	tailRef, err := t.explanationLiteral(head, arity)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&tailRef)) }()
	tail, err := m.RefGet(tailRef)
	if err != nil {
		return err
	}
	rule, err := t.rules.NewRule(head, []ast.ASTHandle{tail}, nil, "")
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, rule.Close()) }()
	return target.Add(rule)
}
