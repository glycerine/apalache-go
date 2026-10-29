package transforms

// Source: src/muz/rel/dl_mk_simple_joins.cpp and .h retained rule-shape
// normalization to at most two positive predicate tails.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type SimpleJoinsPlugin struct {
	manager  *ast.ASTManager
	rules    *base.RuleManager
	priority uint32
	fresh    []ast.ASTHandle
	pins     []ast.ASTRef
	closed   bool
}

func NewSimpleJoinsPlugin(manager *ast.ASTManager, rules *base.RuleManager, priority uint32) *SimpleJoinsPlugin {
	if priority == 0 {
		priority = 1000
	}
	return &SimpleJoinsPlugin{manager: manager, rules: rules, priority: priority}
}

func (p *SimpleJoinsPlugin) Priority() uint32            { return p.priority }
func (p *SimpleJoinsPlugin) CanDestratifyNegation() bool { return false }
func (p *SimpleJoinsPlugin) Cancel()                     {}

func (p *SimpleJoinsPlugin) FreshPredicates() []ast.ASTHandle {
	return append([]ast.ASTHandle(nil), p.fresh...)
}

func (p *SimpleJoinsPlugin) variables(root ast.ASTHandle, result map[uint32]ast.ASTHandle) error {
	kind, err := p.manager.GetKind(root)
	if err != nil {
		return err
	}
	if kind == ast.ASTVar {
		index, x := p.manager.GetVarIndex(root)
		if x != nil {
			return x
		}
		result[index], x = p.manager.GetSort(root)
		return x
	}
	if kind != ast.ASTApp {
		return nil
	}
	count, err := p.manager.GetNumArgs(root)
	if err != nil {
		return err
	}
	for index := uint32(0); index < count; index++ {
		child, x := p.manager.GetArg(root, index)
		if x != nil {
			return x
		}
		if x = p.variables(child, result); x != nil {
			return x
		}
	}
	return nil
}

func (p *SimpleJoinsPlugin) auxiliary(pair, outside []ast.ASTHandle) (ast.ASTHandle, *base.Rule, error) {
	pairVariables := make(map[uint32]ast.ASTHandle)
	live := make(map[uint32]ast.ASTHandle)
	for _, expression := range pair {
		if err := p.variables(expression, pairVariables); err != nil {
			return ast.ASTHandle{}, nil, err
		}
	}
	for _, expression := range outside {
		if err := p.variables(expression, live); err != nil {
			return ast.ASTHandle{}, nil, err
		}
	}
	indices := make([]int, 0)
	for variable := range pairVariables {
		if _, found := live[variable]; found {
			indices = append(indices, int(variable))
		}
	}
	sort.Ints(indices)
	domain := make([]ast.ASTHandle, 0, len(indices))
	arguments := make([]ast.ASTHandle, 0, len(indices))
	for _, raw := range indices {
		variable := uint32(raw)
		domain = append(domain, pairVariables[variable])
		argument, err := p.manager.MkVar(variable, pairVariables[variable])
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		arguments = append(arguments, argument)
	}
	boolean, err := p.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	prefix, err := p.manager.Symbols.Intern("simple-join")
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	declaration, err := p.manager.MkFreshFuncDecl(prefix, usymbol.Symbol{}, domain, boolean)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	if err = p.rules.RegisterPredicate(declaration); err != nil {
		return ast.ASTHandle{}, nil, err
	}
	p.fresh = append(p.fresh, declaration)
	head, err := p.manager.MkApp(declaration, arguments)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	pin, err := p.manager.NewRef(head)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	p.pins = append(p.pins, pin)
	rule, err := p.rules.NewRule(head, pair, nil, "simple-join")
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	return head, rule, nil
}

func (p *SimpleJoinsPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	changed := false
	result, err := base.NewRuleSet(p.rules)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	if err = result.InheritPredicates(source); err != nil {
		return nil, err
	}
	for index := 0; index < count; index++ {
		rule, x := source.Rule(index)
		if x != nil {
			return nil, x
		}
		positive, x := rule.PositiveTailSize()
		if x != nil {
			return nil, errors.Join(x, rule.Close())
		}
		if positive <= 2 {
			x = result.Add(rule)
			x = errors.Join(x, rule.Close())
			if x != nil {
				return nil, x
			}
			continue
		}
		changed = true
		head, x := rule.Head()
		if x != nil {
			return nil, errors.Join(x, rule.Close())
		}
		name, x := rule.Name()
		if x != nil {
			return nil, errors.Join(x, rule.Close())
		}
		tailCount, x := rule.TailSize()
		if x != nil {
			return nil, errors.Join(x, rule.Close())
		}
		tails := make([]ast.ASTHandle, tailCount)
		negated := make([]bool, tailCount)
		for i := range tails {
			tails[i], negated[i], x = rule.Tail(i)
			if x != nil {
				return nil, errors.Join(x, rule.Close())
			}
		}
		for positive > 2 {
			outside := append([]ast.ASTHandle{head}, tails[2:]...)
			auxiliary, definition, y := p.auxiliary(tails[:2], outside)
			if y != nil {
				return nil, errors.Join(y, rule.Close())
			}
			y = result.Add(definition)
			y = errors.Join(y, definition.Close())
			if y != nil {
				return nil, errors.Join(y, rule.Close())
			}
			tails = append([]ast.ASTHandle{auxiliary}, tails[2:]...)
			negated = append([]bool{false}, negated[2:]...)
			positive--
		}
		replacement, x := p.rules.NewRule(head, tails, negated, name)
		if x == nil {
			x = result.Add(replacement)
			x = errors.Join(x, replacement.Close())
		}
		x = errors.Join(x, rule.Close())
		if x != nil {
			return nil, x
		}
	}
	if !changed {
		return nil, nil
	}
	keep = true
	return result, nil
}

func (p *SimpleJoinsPlugin) Close() error {
	if p == nil {
		return base.ErrRuleTransformerOwner
	}
	p.closed = true
	var result error
	for index := len(p.pins) - 1; index >= 0; index-- {
		result = errors.Join(result, p.manager.DelRef(&p.pins[index]))
	}
	p.pins = nil
	p.fresh = nil
	return result
}
