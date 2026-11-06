package rel

// Source: src/muz/rel/dl_mk_explanations.cpp/.h, explanation declaration
// caching, rule transformation, and fact/relation product transfer.

import (
	"bytes"
	"errors"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// ExplanationTransformer owns fresh explanation declarations and borrows the
// rule manager. Transfer methods publish products to the relation manager.
type ExplanationTransformer struct {
	manager          *ast.ASTManager
	rules            *base.RuleManager
	ruleSort         ast.ASTRef
	declarations     map[ast.ASTHandle]ast.ASTRef
	factProducts     map[ast.ASTHandle]*FiniteExplanationProduct
	factManager      *RelationManager
	relationProducts map[ast.ASTHandle]*RelationExplanationProduct
	closed           bool
}

func NewExplanationTransformer(manager *ast.ASTManager, rules *base.RuleManager) (_ *ExplanationTransformer, resultErr error) {
	if manager == nil || rules == nil {
		return nil, ast.ErrASTHandle
	}
	sort, err := manager.MkDatalogRuleSort()
	if err != nil {
		return nil, err
	}
	ref, err := manager.NewRef(sort)
	if err != nil {
		return nil, err
	}
	return &ExplanationTransformer{manager: manager, rules: rules, ruleSort: ref, declarations: make(map[ast.ASTHandle]ast.ASTRef), factProducts: make(map[ast.ASTHandle]*FiniteExplanationProduct), relationProducts: make(map[ast.ASTHandle]*RelationExplanationProduct)}, nil
}

func (t *ExplanationTransformer) open() error {
	if t == nil || t.closed || t.manager == nil || t.rules == nil {
		return ast.ErrASTHandle
	}
	return nil
}

// ExplanationDeclaration returns a borrowed fresh predicate with an added
// rule-sort column. Repeated calls for one predicate return the same handle.
func (t *ExplanationTransformer) ExplanationDeclaration(original ast.ASTHandle) (ast.ASTHandle, error) {
	if err := t.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if ref, found := t.declarations[original]; found {
		return t.manager.RefGet(ref)
	}
	arity, err := t.manager.GetArity(original)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	domain := make([]ast.ASTHandle, 0, arity+1)
	for i := uint32(0); i < arity; i++ {
		sort, err := t.manager.GetDomain(original, i)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		domain = append(domain, sort)
	}
	ruleSort, err := t.manager.RefGet(t.ruleSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	domain = append(domain, ruleSort)
	rangeSort, err := t.manager.GetRange(original)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	name, err := t.manager.GetASTName(original)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	suffix, err := t.manager.Symbols.Intern("expl")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	declaration, err := t.manager.MkFreshFuncDecl(name, suffix, domain, rangeSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := t.manager.NewRef(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = t.rules.RegisterPredicate(declaration); err != nil {
		return ast.ASTHandle{}, errors.Join(err, t.manager.DelRef(&ref))
	}
	t.declarations[original] = ref
	return declaration, nil
}

func (t *ExplanationTransformer) explanationLiteral(literal ast.ASTHandle, index uint32) (ast.ASTRef, error) {
	m := t.manager
	declaration, err := m.GetDecl(literal)
	if err != nil {
		return ast.ASTRef{}, err
	}
	explained, err := t.ExplanationDeclaration(declaration)
	if err != nil {
		return ast.ASTRef{}, err
	}
	count, err := m.GetNumArgs(literal)
	if err != nil {
		return ast.ASTRef{}, err
	}
	arguments := make([]ast.ASTHandle, 0, count+1)
	for i := uint32(0); i < count; i++ {
		arg, err := m.GetArg(literal, i)
		if err != nil {
			return ast.ASTRef{}, err
		}
		arguments = append(arguments, arg)
	}
	sort, err := m.RefGet(t.ruleSort)
	if err != nil {
		return ast.ASTRef{}, err
	}
	variable, err := m.MkVar(index, sort)
	if err != nil {
		return ast.ASTRef{}, err
	}
	arguments = append(arguments, variable)
	value, err := m.MkApp(explained, arguments)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return m.NewRef(value)
}

// ExplanationRule adds an explanation variable to the head and positive
// predicate tails and appends the source rule-record equality.
func (t *ExplanationTransformer) ExplanationRule(rule *base.Rule) (result *base.Rule, resultErr error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, ast.ErrASTHandle
	}
	m := t.manager
	manager, err := rule.ASTManager()
	if err != nil || manager != m {
		return nil, errors.Join(ast.ErrASTHandle, err)
	}
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&refs[i]))
		}
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	hold := func(ref ast.ASTRef, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		refs = append(refs, ref)
		return m.RefGet(ref)
	}
	sorts, err := rule.VariableSorts()
	if err != nil {
		return nil, err
	}
	next := uint32(len(sorts))
	headIndex := next
	next++
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	explainedHead, err := hold(t.explanationLiteral(head, headIndex))
	if err != nil {
		return nil, err
	}
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return nil, err
	}
	total, err := rule.TailSize()
	if err != nil {
		return nil, err
	}
	tails := make([]ast.ASTHandle, 0, total+1)
	negative := make([]bool, 0, total+1)
	ruleArguments := make([]ast.ASTHandle, 0, positive)
	for i := 0; i < total; i++ {
		tail, negated, err := rule.Tail(i)
		if err != nil {
			return nil, err
		}
		if i < positive {
			tail, err = hold(t.explanationLiteral(tail, next))
			if err != nil {
				return nil, err
			}
			next++
			count, err := m.GetNumArgs(tail)
			if err != nil {
				return nil, err
			}
			argument, err := m.GetArg(tail, count-1)
			if err != nil {
				return nil, err
			}
			ruleArguments = append(ruleArguments, argument)
		}
		tails, negative = append(tails, tail), append(negative, negated)
	}
	name, err := rule.NameSymbol()
	if err != nil {
		return nil, err
	}
	if name == (symbol.Symbol{}) {
		var display bytes.Buffer
		if err = t.rules.WriteRuleDisplay(&display, rule); err != nil {
			return nil, err
		}
		name, err = m.Symbols.Intern(strings.TrimRight(display.String(), "\n"))
		if err != nil {
			return nil, err
		}
	}
	record, err := m.MkDatalogRule(name, ruleArguments)
	if err != nil {
		return nil, err
	}
	recordRef, err := m.NewRef(record)
	if err != nil {
		return nil, err
	}
	refs = append(refs, recordRef)
	sort, err := m.RefGet(t.ruleSort)
	if err != nil {
		return nil, err
	}
	variable, err := m.MkVar(headIndex, sort)
	if err != nil {
		return nil, err
	}
	equality, err := m.MkEq(variable, record)
	if err != nil {
		return nil, err
	}
	equalityRef, err := m.NewRef(equality)
	if err != nil {
		return nil, err
	}
	refs = append(refs, equalityRef)
	tails, negative = append(tails, equality), append(negative, false)
	return t.rules.NewRule(explainedHead, tails, negative, "")
}

func (t *ExplanationTransformer) Close() (result error) {
	if t == nil || t.closed {
		return nil
	}
	// Both fact-level and relation-level products are owned by the relation
	// manager under their explained declarations, as source e_rel.
	clear(t.factProducts)
	clear(t.relationProducts)
	t.factManager = nil
	for key, ref := range t.declarations {
		if err := t.manager.DelRef(&ref); err != nil {
			result = errors.Join(result, err)
		} else {
			delete(t.declarations, key)
		}
	}
	if t.ruleSort != (ast.ASTRef{}) {
		result = errors.Join(result, t.manager.DelRef(&t.ruleSort))
	}
	if result == nil {
		t.manager, t.rules, t.declarations, t.factProducts, t.relationProducts, t.closed = nil, nil, nil, nil, nil, true
	}
	return result
}
