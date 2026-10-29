package base

// Source: src/muz/base/hnf.cpp and hnf.h. This file ports the retained
// no-proof Horn-normal-form worklist, disjunctive definition introduction,
// fresh predicate construction, and universal-prefix reconstruction.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

var ErrHornNormalizationOwner = errors.New("muz/base: invalid or closed Horn normalization")

// HornNormalization owns every normalized formula and fresh predicate
// declaration produced by HornNormalizer.Normalize.
type HornNormalization struct {
	manager         *ast.ASTManager
	Formulas        []ast.ASTRef
	FreshPredicates []ast.ASTRef
	closed          bool
}

func (r *HornNormalization) Close() (result error) {
	if r == nil || r.manager == nil {
		return ErrHornNormalizationOwner
	}
	if r.closed {
		return nil
	}
	for i := len(r.Formulas) - 1; i >= 0; i-- {
		result = errors.Join(result, r.manager.DelRef(&r.Formulas[i]))
	}
	for i := len(r.FreshPredicates) - 1; i >= 0; i-- {
		result = errors.Join(result, r.manager.DelRef(&r.FreshPredicates[i]))
	}
	r.Formulas = nil
	r.FreshPredicates = nil
	r.closed = true
	return result
}

type hornPrefix struct {
	sorts []ast.ASTHandle
	names []usymbol.Symbol
}

type hornWork struct {
	formula ast.ASTHandle
	prefix  hornPrefix
}

// HornNormalizer converts formulas into a conjunction represented as a slice
// of Horn formulas. It borrows the AST manager.
type HornNormalizer struct {
	manager *ast.ASTManager
	prefix  usymbol.Symbol
}

func NewHornNormalizer(manager *ast.ASTManager, name string) (*HornNormalizer, error) {
	if manager == nil {
		return nil, ErrHornNormalizationOwner
	}
	if name == "" {
		name = "P"
	}
	prefix, err := manager.Symbols.Intern(name)
	if err != nil {
		return nil, err
	}
	return &HornNormalizer{manager: manager, prefix: prefix}, nil
}

func (n *HornNormalizer) pin(locals *[]ast.ASTRef, handle ast.ASTHandle) error {
	reference, err := n.manager.NewRef(handle)
	if err != nil {
		return err
	}
	*locals = append(*locals, reference)
	return nil
}

func (n *HornNormalizer) predicate(expression ast.ASTHandle) (bool, error) {
	kind, err := n.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	declaration, err := n.manager.GetDecl(expression)
	if err != nil {
		return false, err
	}
	family, err := n.manager.GetFamilyID(declaration)
	if err != nil {
		return false, err
	}
	rangeSort, err := n.manager.GetRange(declaration)
	if err != nil {
		return false, err
	}
	boolean, err := n.manager.MkBoolSort()
	return family == 0 && rangeSort == boolean, err
}

func (n *HornNormalizer) containsPredicate(root ast.ASTHandle) (bool, error) {
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
		kind, err := n.manager.GetKind(current)
		if err != nil {
			return false, err
		}
		switch kind {
		case ast.ASTApp:
			predicate, e := n.predicate(current)
			if e != nil {
				return false, e
			}
			if predicate {
				return true, nil
			}
			count, e := n.manager.GetNumArgs(current)
			if e != nil {
				return false, e
			}
			for i := count; i > 0; i-- {
				argument, x := n.manager.GetArg(current, i-1)
				if x != nil {
					return false, x
				}
				stack = append(stack, argument)
			}
		case ast.ASTQuantifier:
			body, e := n.manager.GetQuantifierBody(current)
			if e != nil {
				return false, e
			}
			stack = append(stack, body)
		}
	}
	return false, nil
}

func (n *HornNormalizer) operator(expression ast.ASTHandle) (int32, bool, error) {
	kind, err := n.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return -1, false, err
	}
	operator, err := n.manager.GetDeclKind(expression)
	return operator, err == nil, err
}

func (n *HornNormalizer) stripForall(formula ast.ASTHandle) (ast.ASTHandle, hornPrefix, error) {
	current := formula
	prefix := hornPrefix{}
	for {
		kind, err := n.manager.GetKind(current)
		if err != nil || kind != ast.ASTQuantifier {
			return current, prefix, err
		}
		forall, err := n.manager.IsForall(current)
		if err != nil {
			return ast.ASTHandle{}, hornPrefix{}, err
		}
		if !forall {
			return current, prefix, nil
		}
		count, err := n.manager.GetNumDeclarations(current)
		if err != nil {
			return ast.ASTHandle{}, hornPrefix{}, err
		}
		for i := uint32(0); i < count; i++ {
			sort, e := n.manager.GetDeclarationSort(current, i)
			if e != nil {
				return ast.ASTHandle{}, hornPrefix{}, e
			}
			name, e := n.manager.GetDeclarationName(current, i)
			if e != nil {
				return ast.ASTHandle{}, hornPrefix{}, e
			}
			prefix.sorts = append(prefix.sorts, sort)
			prefix.names = append(prefix.names, name)
		}
		current, err = n.manager.GetQuantifierBody(current)
		if err != nil {
			return ast.ASTHandle{}, hornPrefix{}, err
		}
	}
}

func (n *HornNormalizer) bind(prefix hornPrefix, body ast.ASTHandle, locals *[]ast.ASTRef) (ast.ASTHandle, error) {
	if len(prefix.sorts) == 0 {
		return body, nil
	}
	result, err := n.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: prefix.sorts, Names: prefix.names, Body: body})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = n.pin(locals, result); err != nil {
		return ast.ASTHandle{}, err
	}
	return result, nil
}

func (n *HornNormalizer) freshHead(expression ast.ASTHandle, result *HornNormalization, locals *[]ast.ASTRef) (ast.ASTHandle, error) {
	used, err := ast.NewASTUsedVars(n.manager)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = used.Collect(expression); err != nil {
		return ast.ASTHandle{}, err
	}
	domain := make([]ast.ASTHandle, 0, used.NumVars())
	arguments := make([]ast.ASTHandle, 0, used.NumVars())
	for i := uint32(0); i < used.MaxFoundVarIndexPlusOne(); i++ {
		sort := used.Contains(i)
		if sort.IsNull() {
			continue
		}
		domain = append(domain, sort)
		variable, e := n.manager.MkVar(i, sort)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		arguments = append(arguments, variable)
	}
	boolean, err := n.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	declaration, err := n.manager.MkFreshFuncDecl(n.prefix, usymbol.Symbol{}, domain, boolean)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	declarationRef, err := n.manager.NewRef(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result.FreshPredicates = append(result.FreshPredicates, declarationRef)
	head, err := n.manager.MkApp(declaration, arguments)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = n.pin(locals, head); err != nil {
		return ast.ASTHandle{}, err
	}
	return head, nil
}

func (n *HornNormalizer) implication(body []ast.ASTHandle, head ast.ASTHandle, locals *[]ast.ASTRef) (ast.ASTHandle, error) {
	if len(body) == 0 {
		return head, nil
	}
	antecedent := body[0]
	var err error
	if len(body) > 1 {
		antecedent, err = n.manager.MkAndArgs(body)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if err = n.pin(locals, antecedent); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	formula, err := n.manager.MkImplies(antecedent, head)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = n.pin(locals, formula); err != nil {
		return ast.ASTHandle{}, err
	}
	return formula, nil
}

// Normalize returns the conjunction of Horn formulas and its fresh predicate
// declarations. Proof production and general nested quantifier hoisting remain
// outside this increment.
func (n *HornNormalizer) Normalize(formula ast.ASTHandle) (result *HornNormalization, resultErr error) {
	if n == nil || n.manager == nil {
		return nil, ErrHornNormalizationOwner
	}
	result = &HornNormalization{manager: n.manager}
	failed := true
	defer func() {
		if failed || resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	var locals []ast.ASTRef
	defer func() {
		for i := len(locals) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, n.manager.DelRef(&locals[i]))
		}
	}()
	if err := n.pin(&locals, formula); err != nil {
		return result, err
	}
	work := []hornWork{{formula: formula}}
	memo := make(map[ast.ASTHandle]ast.ASTHandle)
	for len(work) != 0 {
		last := len(work) - 1
		item := work[last]
		work = work[:last]
		core, prefix, err := n.stripForall(item.formula)
		if err != nil {
			return result, err
		}
		if len(item.prefix.sorts) != 0 {
			prefix = item.prefix
		}
		head := core
		var body []ast.ASTHandle
		kind, err := n.manager.GetKind(core)
		if err != nil {
			return result, err
		}
		if kind == ast.ASTApp {
			operator, e := n.manager.GetDeclKind(core)
			if e != nil {
				return result, e
			}
			for operator == ast.ASTOpImplies {
				antecedent, x := n.manager.GetArg(head, 0)
				if x != nil {
					return result, x
				}
				body = append(body, antecedent)
				head, x = n.manager.GetArg(head, 1)
				if x != nil {
					return result, x
				}
				headKind, x := n.manager.GetKind(head)
				if x != nil || headKind != ast.ASTApp {
					break
				}
				operator, x = n.manager.GetDeclKind(head)
				if x != nil {
					return result, x
				}
			}
		}
		body, err = n.manager.FlattenAnd(body)
		if err != nil {
			return result, err
		}
		if len(body) == 1 {
			operator, application, e := n.operator(body[0])
			if e != nil {
				return result, e
			}
			contains, e := n.containsPredicate(body[0])
			if e != nil {
				return result, e
			}
			if application && operator == ast.ASTOpOr && contains {
				count, e := n.manager.GetNumArgs(body[0])
				if e != nil {
					return result, e
				}
				for i := uint32(0); i < count; i++ {
					disjunct, x := n.manager.GetArg(body[0], i)
					if x != nil {
						return result, x
					}
					definition, x := n.implication([]ast.ASTHandle{disjunct}, head, &locals)
					if x != nil {
						return result, x
					}
					definition, x = n.bind(prefix, definition, &locals)
					if x != nil {
						return result, x
					}
					work = append(work, hornWork{formula: definition, prefix: prefix})
				}
				continue
			}
		}
		for i, expression := range body {
			contains, e := n.containsPredicate(expression)
			if e != nil {
				return result, e
			}
			if !contains {
				continue
			}
			operator, application, e := n.operator(expression)
			if e != nil {
				return result, e
			}
			if !application {
				continue
			}
			var disjuncts []ast.ASTHandle
			negate := false
			switch operator {
			case ast.ASTOpOr:
				count, x := n.manager.GetNumArgs(expression)
				if x != nil {
					return result, x
				}
				for j := uint32(0); j < count; j++ {
					argument, x := n.manager.GetArg(expression, j)
					if x != nil {
						return result, x
					}
					disjuncts = append(disjuncts, argument)
				}
			case ast.ASTOpNot:
				argument, x := n.manager.GetArg(expression, 0)
				if x != nil {
					return result, x
				}
				if op, app, x := n.operator(argument); x == nil && app && op == ast.ASTOpAnd {
					disjuncts, e = n.manager.FlattenAnd([]ast.ASTHandle{argument})
					negate = true
				} else if x != nil {
					return result, x
				}
			case ast.ASTOpImplies:
				left, x := n.manager.GetArg(expression, 0)
				if x != nil {
					return result, x
				}
				right, x := n.manager.GetArg(expression, 1)
				if x != nil {
					return result, x
				}
				notLeft, x := n.manager.MkNot(left)
				if x != nil {
					return result, x
				}
				if x = n.pin(&locals, notLeft); x != nil {
					return result, x
				}
				disjuncts = []ast.ASTHandle{notLeft, right}
			}
			if len(disjuncts) == 0 {
				continue
			}
			fresh, found := memo[expression]
			if !found {
				fresh, e = n.freshHead(expression, result, &locals)
				if e != nil {
					return result, e
				}
				memo[expression] = fresh
				for _, disjunct := range disjuncts {
					if negate {
						disjunct, e = n.manager.MkNot(disjunct)
						if e != nil {
							return result, e
						}
						if e = n.pin(&locals, disjunct); e != nil {
							return result, e
						}
					}
					definition, x := n.implication([]ast.ASTHandle{disjunct}, fresh, &locals)
					if x != nil {
						return result, x
					}
					definition, x = n.bind(prefix, definition, &locals)
					if x != nil {
						return result, x
					}
					work = append(work, hornWork{formula: definition, prefix: prefix})
				}
			}
			body[i] = fresh
		}
		normalized, err := n.implication(body, head, &locals)
		if err != nil {
			return result, err
		}
		normalized, err = n.bind(prefix, normalized, &locals)
		if err != nil {
			return result, err
		}
		reference, err := n.manager.NewRef(normalized)
		if err != nil {
			return result, err
		}
		result.Formulas = append(result.Formulas, reference)
	}
	failed = false
	return result, nil
}

// AddFormula normalizes a formula, registers introduced predicates, converts
// every Horn conjunct to a Rule, and retains it in set.
func (m *RuleManager) AddFormula(set *RuleSet, formula ast.ASTHandle, name string) (resultErr error) {
	if err := m.open(); err != nil {
		return err
	}
	if set == nil || set.manager != m {
		return ErrRuleSetOwner
	}
	normalizer, err := NewHornNormalizer(m.manager, name)
	if err != nil {
		return err
	}
	normalized, err := normalizer.Normalize(formula)
	if normalized != nil {
		defer func() { resultErr = errors.Join(resultErr, normalized.Close()) }()
	}
	if err != nil {
		return err
	}
	for _, reference := range normalized.FreshPredicates {
		declaration, e := m.manager.RefGet(reference)
		if e != nil {
			return e
		}
		if e = m.RegisterPredicate(declaration); e != nil {
			return e
		}
	}
	for _, reference := range normalized.Formulas {
		horn, e := m.manager.RefGet(reference)
		if e != nil {
			return e
		}
		rule, e := m.NewRuleFromFormula(horn, name)
		if e != nil {
			return e
		}
		if e = set.Add(rule); e != nil {
			return errors.Join(e, rule.Close())
		}
		if e = rule.Close(); e != nil {
			return e
		}
	}
	return nil
}
