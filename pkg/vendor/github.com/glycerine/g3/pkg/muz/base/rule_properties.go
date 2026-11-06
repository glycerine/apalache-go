package base

// Source: src/muz/base/rule_properties.h; src/muz/base/rule_properties.cpp.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

var ErrRuleProperties = errors.New("muz/base: invalid or closed rule properties")

type RulePropertyKind uint8

const (
	RulePropertyQuantifier RulePropertyKind = iota + 1
	RulePropertyUninterpreted
	RulePropertyNested
	RulePropertyInfiniteSort
	RulePropertyExistentialTail
	RulePropertyNegativePredicate
)

type RulePropertyError struct {
	Kind     RulePropertyKind
	RuleName string
	Symbol   string
	// SourceMessage preserves the source diagnostic while the offending rule
	// is still owned by the property collector.
	SourceMessage string
}

func (e *RulePropertyError) Error() string {
	suffix := ""
	if e.RuleName != "" {
		suffix = " in rule " + e.RuleName
	}
	switch e.Kind {
	case RulePropertyQuantifier:
		return "muz/base: cannot process quantifier" + suffix
	case RulePropertyUninterpreted:
		if e.Symbol != "" {
			return "muz/base: uninterpreted '" + e.Symbol + "'" + suffix
		}
		return "muz/base: uninterpreted symbol" + suffix
	case RulePropertyNested:
		return "muz/base: rule contains nested predicates" + suffix
	case RulePropertyInfiniteSort:
		return "muz/base: rule contains infinite sorts" + suffix
	case RulePropertyExistentialTail:
		return "muz/base: recursive predicate occurs nested in the body" + suffix
	case RulePropertyNegativePredicate:
		return "muz/base: rule contains negative predicate" + suffix
	default:
		return ErrRuleProperties.Error()
	}
}

// RuleProperties owns the rule references retained by the latest collection.
// RuleManager supplies the source predicate classifier and AST manager.
type RuleProperties struct {
	self          *RuleProperties
	rules         *RuleManager
	manager       *ast.ASTManager
	owned         []*Rule
	quantifier    *Rule
	uninterpreted *Rule
	uninterpDecl  ast.ASTHandle
	nested        []*Rule
	negative      *Rule
	infinite      *Rule
	generateProof bool
	closed        bool
}

// SetGenerateProof enables source rule-property collection's creation of
// asserted proofs for rules that do not already have one.
func (p *RuleProperties) SetGenerateProof(enabled bool) error {
	if err := p.open(); err != nil {
		return err
	}
	p.generateProof = enabled
	return nil
}

func NewRuleProperties(rules *RuleManager) (*RuleProperties, error) {
	if rules == nil {
		return nil, ErrRuleProperties
	}
	if err := rules.open(); err != nil {
		return nil, err
	}
	p := &RuleProperties{rules: rules, manager: rules.manager}
	p.self = p
	return p, nil
}

func (p *RuleProperties) open() error {
	if p == nil || p.self != p || p.closed || p.rules == nil || p.manager == nil {
		return ErrRuleProperties
	}
	return p.rules.open()
}

func (p *RuleProperties) clearState() {
	p.quantifier = nil
	p.uninterpreted = nil
	p.uninterpDecl = ast.ASTHandle{}
	p.nested = nil
	p.negative = nil
	p.infinite = nil
}

func (p *RuleProperties) Reset() error {
	if err := p.open(); err != nil {
		return err
	}
	for len(p.owned) != 0 {
		i := len(p.owned) - 1
		if err := p.owned[i].Close(); err != nil {
			return err
		}
		p.owned = p.owned[:i]
	}
	p.clearState()
	return nil
}

func (p *RuleProperties) appendNested(rule *Rule) {
	if len(p.nested) == 0 || p.nested[len(p.nested)-1] != rule {
		p.nested = append(p.nested, rule)
	}
}

func (p *RuleProperties) collectExpression(rule *Rule, root ast.ASTHandle, visited map[ast.ASTHandle]bool) error {
	stack := []ast.ASTHandle{root}
	for len(stack) != 0 {
		i := len(stack) - 1
		expression := stack[i]
		stack = stack[:i]
		if visited[expression] {
			continue
		}
		visited[expression] = true
		kind, err := p.manager.GetKind(expression)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTVar:
			continue
		case ast.ASTQuantifier:
			if p.quantifier == nil {
				p.quantifier = rule
			}
			body, e := p.manager.GetQuantifierBody(expression)
			if e != nil {
				return e
			}
			stack = append(stack, body)
			patterns, e := p.manager.GetNumPatterns(expression)
			if e != nil {
				return e
			}
			for j := uint32(0); j < patterns; j++ {
				child, x := p.manager.GetPattern(expression, j)
				if x != nil {
					return x
				}
				stack = append(stack, child)
			}
			noPatterns, e := p.manager.GetNumNoPatterns(expression)
			if e != nil {
				return e
			}
			for j := uint32(0); j < noPatterns; j++ {
				child, x := p.manager.GetNoPattern(expression, j)
				if x != nil {
					return x
				}
				stack = append(stack, child)
			}
		case ast.ASTApp:
			predicate, e := p.rules.isPredicate(expression)
			if e != nil {
				return e
			}
			declaration, e := p.manager.GetDecl(expression)
			if e != nil {
				return e
			}
			if predicate {
				p.appendNested(rule)
			} else {
				family, x := p.manager.GetFamilyID(declaration)
				if x != nil {
					return x
				}
				if family == ast.NullFamilyID {
					rangeSort, x := p.manager.GetRange(declaration)
					if x != nil {
						return x
					}
					isRuleSort, x := p.manager.IsDatalogRuleSort(rangeSort)
					if x != nil {
						return x
					}
					if !isRuleSort && p.uninterpreted == nil {
						p.uninterpreted, p.uninterpDecl = rule, declaration
					}
				}
				if datatypeFamily, x := p.manager.DatatypeFamilyID(); x == nil && family == datatypeFamily {
					declKind, x := p.manager.GetDeclKind(declaration)
					if x != nil {
						return x
					}
					if declKind == ast.ASTDatatypeAccessor {
						argument, x := p.manager.GetArg(expression, 0)
						if x != nil {
							return x
						}
						sort, x := p.manager.GetSort(argument)
						if x != nil {
							return x
						}
						constructors, x := p.manager.DatatypeConstructors(sort)
						if x != nil {
							return x
						}
						if len(constructors) > 1 && p.uninterpreted == nil {
							p.uninterpreted, p.uninterpDecl = rule, declaration
						}
					}
				}
			}
			arguments, e := p.manager.GetNumArgs(expression)
			if e != nil {
				return e
			}
			for j := arguments; j > 0; j-- {
				child, x := p.manager.GetArg(expression, j-1)
				if x != nil {
					return x
				}
				stack = append(stack, child)
			}
		}
	}
	return nil
}

func (p *RuleProperties) Collect(set *RuleSet) error {
	if err := p.open(); err != nil {
		return err
	}
	if set == nil || set.manager != p.rules {
		return ErrRuleProperties
	}
	if err := p.Reset(); err != nil {
		return err
	}
	count, err := set.Len()
	if err != nil {
		return err
	}
	visited := make(map[ast.ASTHandle]bool)
	for i := 0; i < count; i++ {
		rule, e := set.Rule(i)
		if e != nil {
			return e
		}
		p.owned = append(p.owned, rule)
		if p.negative == nil {
			negative, x := rule.HasNegation()
			if x != nil {
				return x
			}
			if negative {
				p.negative = rule
			}
		}
		uninterpreted, e := rule.UninterpretedTailSize()
		if e != nil {
			return e
		}
		tails, e := rule.TailSize()
		if e != nil {
			return e
		}
		for j := uninterpreted; j < tails; j++ {
			tail, _, x := rule.Tail(j)
			if x != nil {
				return x
			}
			if x = p.collectExpression(rule, tail, visited); x != nil {
				return x
			}
		}
		if p.generateProof {
			if e = p.rules.MkRuleAssertedProof(rule); e != nil {
				return e
			}
		}
		if p.infinite == nil {
			head, x := rule.HeadDeclaration()
			if x != nil {
				return x
			}
			arity, x := p.manager.GetArity(head)
			if x != nil {
				return x
			}
			for column := uint32(0); column < arity; column++ {
				sort, x := p.manager.GetDomain(head, column)
				if x != nil {
					return x
				}
				size, x := p.manager.GetSortSize(sort)
				if x != nil {
					return x
				}
				ruleSort, x := p.manager.IsDatalogRuleSort(sort)
				if x != nil {
					return x
				}
				if !size.IsFinite() && !ruleSort {
					p.infinite = rule
					break
				}
			}
		}
	}
	return nil
}

func rulePropertyName(rule *Rule) string {
	if rule == nil {
		return ""
	}
	name, _ := rule.NameText()
	return name
}

func (p *RuleProperties) CheckQuantifierFree() error {
	if err := p.open(); err != nil {
		return err
	}
	if p.quantifier != nil {
		var display strings.Builder
		if err := p.rules.WriteRuleDisplay(&display, p.quantifier); err != nil {
			return err
		}
		return &RulePropertyError{Kind: RulePropertyQuantifier, RuleName: rulePropertyName(p.quantifier), SourceMessage: "cannot process quantifier in rule " + display.String()}
	}
	return nil
}

func (p *RuleProperties) CheckUninterpretedFree() error {
	if err := p.open(); err != nil {
		return err
	}
	if p.uninterpreted == nil {
		return nil
	}
	name := ""
	if symbol, err := p.manager.GetASTName(p.uninterpDecl); err == nil {
		name, _ = p.manager.Symbols.Display(symbol)
	}
	var display strings.Builder
	if err := p.rules.WriteRuleDisplay(&display, p.uninterpreted); err != nil {
		return err
	}
	return &RulePropertyError{Kind: RulePropertyUninterpreted, RuleName: rulePropertyName(p.uninterpreted), Symbol: name, SourceMessage: "Uninterpreted '" + name + "' in " + display.String()}
}

func (p *RuleProperties) CheckNestedFree() error {
	if err := p.open(); err != nil {
		return err
	}
	if len(p.nested) != 0 {
		var display strings.Builder
		if err := p.rules.WriteRuleDisplay(&display, p.nested[0]); err != nil {
			return err
		}
		return &RulePropertyError{Kind: RulePropertyNested, RuleName: rulePropertyName(p.nested[0]), SourceMessage: "Rule contains nested predicates " + display.String()}
	}
	return nil
}

func (p *RuleProperties) CheckInfiniteSorts() error {
	if err := p.open(); err != nil {
		return err
	}
	if p.infinite != nil {
		var display strings.Builder
		if err := p.rules.WriteRuleDisplay(&display, p.infinite); err != nil {
			return err
		}
		return &RulePropertyError{Kind: RulePropertyInfiniteSort, RuleName: rulePropertyName(p.infinite), SourceMessage: "Rule contains infinite sorts in rule " + display.String()}
	}
	return nil
}

func (p *RuleProperties) CheckNegativePredicates() error {
	if err := p.open(); err != nil {
		return err
	}
	if p.negative != nil {
		var display strings.Builder
		if err := p.rules.WriteRuleDisplay(&display, p.negative); err != nil {
			return err
		}
		return &RulePropertyError{Kind: RulePropertyNegativePredicate, RuleName: rulePropertyName(p.negative), SourceMessage: "Rule contains negative predicate " + display.String()}
	}
	return nil
}

func (p *RuleProperties) containsPredicate(root ast.ASTHandle) (bool, error) {
	stack := []ast.ASTHandle{root}
	seen := make(map[ast.ASTHandle]bool)
	for len(stack) != 0 {
		i := len(stack) - 1
		e := stack[i]
		stack = stack[:i]
		if seen[e] {
			continue
		}
		seen[e] = true
		kind, err := p.manager.GetKind(e)
		if err != nil {
			return false, err
		}
		switch kind {
		case ast.ASTApp:
			predicate, x := p.rules.isPredicate(e)
			if x != nil {
				return false, x
			}
			if predicate {
				return true, nil
			}
			n, x := p.manager.GetNumArgs(e)
			if x != nil {
				return false, x
			}
			for j := uint32(0); j < n; j++ {
				child, y := p.manager.GetArg(e, j)
				if y != nil {
					return false, y
				}
				stack = append(stack, child)
			}
		case ast.ASTQuantifier:
			body, x := p.manager.GetQuantifierBody(e)
			if x != nil {
				return false, x
			}
			stack = append(stack, body)
		}
	}
	return false, nil
}

func (p *RuleProperties) CheckExistentialTail() error {
	if err := p.open(); err != nil {
		return err
	}
	// Source appends all interpreted tails before consuming the shared LIFO
	// worklist, then checks its candidate expressions in insertion order.
	type candidate struct {
		expr ast.ASTHandle
		rule *Rule
	}
	var todo, toCheck []candidate
	for _, rule := range p.nested {
		uninterpreted, err := rule.UninterpretedTailSize()
		if err != nil {
			return err
		}
		tails, err := rule.TailSize()
		if err != nil {
			return err
		}
		for i := uninterpreted; i < tails; i++ {
			tail, _, err := rule.Tail(i)
			if err != nil {
				return err
			}
			todo = append(todo, candidate{tail, rule})
		}
	}
	visited := make(map[ast.ASTHandle]bool)
	for len(todo) != 0 {
		i := len(todo) - 1
		item := todo[i]
		todo = todo[:i]
		e := item.expr
		if visited[e] {
			continue
		}
		visited[e] = true
		kind, err := p.manager.GetKind(e)
		if err != nil {
			return err
		}
		if kind == ast.ASTQuantifier {
			body, err := p.manager.GetQuantifierBody(e)
			if err != nil {
				return err
			}
			toCheck = append(toCheck, candidate{body, item.rule})
			continue
		}
		if kind != ast.ASTApp {
			toCheck = append(toCheck, item)
			continue
		}
		predicate, err := p.rules.isPredicate(e)
		if err != nil {
			return err
		}
		if predicate {
			continue
		}
		op, err := p.manager.GetDeclKind(e)
		if err != nil {
			return err
		}
		n, err := p.manager.GetNumArgs(e)
		if err != nil {
			return err
		}
		switch op {
		case ast.ASTOpAnd, ast.ASTOpOr:
			for j := uint32(0); j < n; j++ {
				child, err := p.manager.GetArg(e, j)
				if err != nil {
					return err
				}
				todo = append(todo, candidate{child, item.rule})
			}
		case ast.ASTOpImplies:
			if n != 2 {
				return fmt.Errorf("%w: malformed implication", ErrRuleProperties)
			}
			left, err := p.manager.GetArg(e, 0)
			if err != nil {
				return err
			}
			right, err := p.manager.GetArg(e, 1)
			if err != nil {
				return err
			}
			toCheck = append(toCheck, candidate{left, item.rule})
			todo = append(todo, candidate{right, item.rule})
		case ast.ASTOpEq, ast.ASTOpIff:
			if n != 2 {
				toCheck = append(toCheck, item)
				continue
			}
			left, err := p.manager.GetArg(e, 0)
			if err != nil {
				return err
			}
			right, err := p.manager.GetArg(e, 1)
			if err != nil {
				return err
			}
			if p.manager.IsTrue(left) {
				todo = append(todo, candidate{right, item.rule})
			} else if p.manager.IsTrue(right) {
				todo = append(todo, candidate{left, item.rule})
			} else {
				toCheck = append(toCheck, item)
			}
		default:
			toCheck = append(toCheck, item)
		}
	}
	for _, item := range toCheck {
		found, err := p.containsPredicate(item.expr)
		if err != nil {
			return err
		}
		if found {
			display, err := p.rules.ruleHeadSourceDisplay(item.expr)
			if err != nil {
				return err
			}
			return &RulePropertyError{Kind: RulePropertyExistentialTail, RuleName: rulePropertyName(item.rule), SourceMessage: "recursive predicate " + display + " occurs nested in the body of a rule"}
		}
	}
	return nil
}

func (p *RuleProperties) Close() error {
	if p == nil || p.self != p {
		return ErrRuleProperties
	}
	if p.closed {
		return nil
	}
	if err := p.Reset(); err != nil {
		return err
	}
	p.closed = true
	p.rules = nil
	p.manager = nil
	return nil
}
