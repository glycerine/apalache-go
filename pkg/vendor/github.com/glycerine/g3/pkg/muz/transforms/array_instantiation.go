package transforms

// Source: src/muz/transforms/dl_mk_array_instantiation.cpp/.h retained
// predicate-cell instantiation for array Horn clauses.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

type ArrayInstantiationPlugin struct {
	manager          *ast.ASTManager
	rules            *base.RuleManager
	priority         uint32
	quantifiers      uint32
	enforce          bool
	oldToNew         map[ast.ASTHandle]ast.ASTHandle
	selects          map[ast.ASTHandle][]ast.ASTHandle
	equivalence      map[ast.ASTHandle]ast.ASTHandle
	equivalenceOrder []ast.ASTHandle
	selectVariables  map[ast.ASTHandle]ast.ASTHandle
	selectOrder      []ast.ASTHandle
	nextVariable     uint32
	closed           bool
}

func NewArrayInstantiationPlugin(manager *ast.ASTManager, rules *base.RuleManager, quantifiers uint32, enforce bool, priority uint32) *ArrayInstantiationPlugin {
	return &ArrayInstantiationPlugin{manager: manager, rules: rules, quantifiers: quantifiers, enforce: enforce, priority: priority, oldToNew: make(map[ast.ASTHandle]ast.ASTHandle)}
}

func (p *ArrayInstantiationPlugin) Priority() uint32            { return p.priority }
func (p *ArrayInstantiationPlugin) CanDestratifyNegation() bool { return false }
func (p *ArrayInstantiationPlugin) Cancel()                     {}

func (p *ArrayInstantiationPlugin) FreshPredicates() map[ast.ASTHandle]ast.ASTHandle {
	result := make(map[ast.ASTHandle]ast.ASTHandle, len(p.oldToNew))
	for old, fresh := range p.oldToNew {
		result[old] = fresh
	}
	return result
}

func (p *ArrayInstantiationPlugin) isArray(sort ast.ASTHandle) (bool, error) {
	family, err := p.manager.GetFamilyID(sort)
	if err != nil {
		return false, err
	}
	array, err := p.manager.ArrayFamilyID()
	return family == array, err
}

func (p *ArrayInstantiationPlugin) arrayApplication(expression ast.ASTHandle, operation int32) (bool, error) {
	kind, err := p.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := p.manager.GetFamilyID(expression)
	if err != nil {
		return false, err
	}
	array, err := p.manager.ArrayFamilyID()
	if err != nil || family != array {
		return false, err
	}
	op, err := p.manager.GetDeclKind(expression)
	return op == operation, err
}

func (p *ArrayInstantiationPlugin) selectTerm(array ast.ASTHandle, indices []ast.ASTHandle) (ast.ASTHandle, error) {
	family, err := p.manager.ArrayFamilyID()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arguments := append([]ast.ASTHandle{array}, indices...)
	return p.manager.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, arguments)
}

func (p *ArrayInstantiationPlugin) maximumVariable(rule *base.Rule) (uint32, error) {
	used, err := ast.NewASTUsedVars(p.manager)
	if err != nil {
		return 0, err
	}
	head, err := rule.Head()
	if err == nil {
		err = used.Process(head)
	}
	count, x := rule.TailSize()
	if err == nil {
		err = x
	}
	for i := 0; err == nil && i < count; i++ {
		var tail ast.ASTHandle
		tail, _, err = rule.Tail(i)
		if err == nil {
			err = used.Process(tail)
		}
	}
	return used.MaxFoundVarIndexPlusOne(), err
}

func (p *ArrayInstantiationPlugin) representative(value ast.ASTHandle) ast.ASTHandle {
	parent, found := p.equivalence[value]
	if !found {
		p.equivalence[value] = value
		p.equivalenceOrder = append(p.equivalenceOrder, value)
		return value
	}
	for parent != p.equivalence[parent] {
		parent = p.equivalence[parent]
	}
	return parent
}

func (p *ArrayInstantiationPlugin) merge(left, right ast.ASTHandle) {
	l, r := p.representative(left), p.representative(right)
	if l != r {
		p.equivalence[l] = r
	}
}

func (p *ArrayInstantiationPlugin) retrieveSelects(expression ast.ASTHandle) error {
	kind, err := p.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return err
	}
	count, err := p.manager.GetNumArgs(expression)
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		argument, x := p.manager.GetArg(expression, i)
		if x != nil {
			return x
		}
		if x = p.retrieveSelects(argument); x != nil {
			return x
		}
	}
	selectTerm, err := p.arrayApplication(expression, ast.ASTArrayOpSelect)
	if err != nil {
		return err
	}
	if selectTerm {
		array, x := p.manager.GetArg(expression, 0)
		if x != nil {
			return x
		}
		p.representative(array)
		p.selects[array] = append(p.selects[array], expression)
	}
	store, err := p.arrayApplication(expression, ast.ASTArrayOpStore)
	if err != nil {
		return err
	}
	if store {
		baseArray, x := p.manager.GetArg(expression, 0)
		if x != nil {
			return x
		}
		p.merge(expression, baseArray)
		return nil
	}
	family, err := p.manager.GetFamilyID(expression)
	if err != nil {
		return err
	}
	if family == p.manager.BasicFamilyID() {
		op, x := p.manager.GetDeclKind(expression)
		if x != nil {
			return x
		}
		if op == ast.ASTOpEq {
			left, x := p.manager.GetArg(expression, 0)
			if x != nil {
				return x
			}
			right, x := p.manager.GetArg(expression, 1)
			if x != nil {
				return x
			}
			sort, x := p.manager.GetSort(left)
			if x != nil {
				return x
			}
			array, x := p.isArray(sort)
			if x != nil {
				return x
			}
			if array {
				p.merge(left, right)
			}
		}
	}
	return nil
}

func (p *ArrayInstantiationPlugin) linkedSelects(array ast.ASTHandle) ([]ast.ASTHandle, error) {
	root := p.representative(array)
	var result []ast.ASTHandle
	for _, member := range p.equivalenceOrder {
		if p.representative(member) != root {
			continue
		}
		for _, source := range p.selects[member] {
			count, err := p.manager.GetNumArgs(source)
			if err != nil {
				return nil, err
			}
			indices := make([]ast.ASTHandle, 0, count-1)
			for i := uint32(1); i < count; i++ {
				argument, x := p.manager.GetArg(source, i)
				if x != nil {
					return nil, x
				}
				indices = append(indices, argument)
			}
			rewritten, err := p.selectTerm(array, indices)
			if err != nil {
				return nil, err
			}
			result = append(result, rewritten)
		}
	}
	if len(result) != 0 {
		return result, nil
	}
	sort, err := p.manager.GetSort(array)
	if err != nil {
		return nil, err
	}
	arity, err := p.manager.ArrayArity(sort)
	if err != nil {
		return nil, err
	}
	indices := make([]ast.ASTHandle, arity)
	for i := uint32(0); i < arity; i++ {
		indexSort, x := p.manager.ArrayDomain(sort, i)
		if x != nil {
			return nil, x
		}
		indices[i], x = p.manager.MkVar(p.nextVariable, indexSort)
		if x != nil {
			return nil, x
		}
		p.nextVariable++
	}
	selectTerm, err := p.selectTerm(array, indices)
	if err != nil {
		return nil, err
	}
	return []ast.ASTHandle{selectTerm}, nil
}

func (p *ArrayInstantiationPlugin) selectVariable(selectTerm ast.ASTHandle) (ast.ASTHandle, error) {
	if variable, found := p.selectVariables[selectTerm]; found {
		return variable, nil
	}
	sort, err := p.manager.GetSort(selectTerm)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	variable, err := p.manager.MkVar(p.nextVariable, sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	p.nextVariable++
	p.selectVariables[selectTerm] = variable
	p.selectOrder = append(p.selectOrder, selectTerm)
	return variable, nil
}

func (p *ArrayInstantiationPlugin) createPredicate(source, destination *base.RuleSet, application ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
	old, err := p.manager.GetDecl(application)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	complete := append([]ast.ASTHandle(nil), arguments...)
	for _, argument := range arguments {
		selectTerm, x := p.arrayApplication(argument, ast.ASTArrayOpSelect)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		if !selectTerm {
			continue
		}
		count, x := p.manager.GetNumArgs(argument)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		for i := uint32(1); i < count; i++ {
			index, y := p.manager.GetArg(argument, i)
			if y != nil {
				return ast.ASTHandle{}, y
			}
			complete = append(complete, index)
		}
	}
	for i, argument := range complete {
		selectTerm, x := p.arrayApplication(argument, ast.ASTArrayOpSelect)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		if selectTerm {
			complete[i], x = p.selectVariable(argument)
			if x != nil {
				return ast.ASTHandle{}, x
			}
		}
	}
	derived := p.oldToNew[old]
	if derived.IsNull() {
		name, x := p.manager.GetASTName(old)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		text, x := p.manager.Symbols.String(name)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		name, x = p.manager.Symbols.Intern(text + "!inst")
		if x != nil {
			return ast.ASTHandle{}, x
		}
		domain := make([]ast.ASTHandle, len(complete))
		for i, argument := range complete {
			domain[i], x = p.manager.GetSort(argument)
			if x != nil {
				return ast.ASTHandle{}, x
			}
		}
		rangeSort, x := p.manager.GetRange(old)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		derived, x = p.manager.MkFuncDecl(name, domain, rangeSort, ast.ASTFuncDeclInfo{})
		if x != nil {
			return ast.ASTHandle{}, x
		}
		if x = p.rules.RegisterPredicate(derived); x != nil {
			return ast.ASTHandle{}, x
		}
		if x = destination.InheritPredicate(source, old, derived); x != nil {
			return ast.ASTHandle{}, x
		}
		p.oldToNew[old] = derived
	}
	return p.manager.MkApp(derived, complete)
}

func (p *ArrayInstantiationPlugin) createHead(source, destination *base.RuleSet, head ast.ASTHandle) (ast.ASTHandle, error) {
	count, err := p.manager.GetNumArgs(head)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arguments := make([]ast.ASTHandle, 0)
	for i := uint32(0); i < count; i++ {
		argument, x := p.manager.GetArg(head, i)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		sort, x := p.manager.GetSort(argument)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		array, x := p.isArray(sort)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		if !array {
			arguments = append(arguments, argument)
			continue
		}
		arity, x := p.manager.ArrayArity(sort)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		for k := uint32(0); k < p.quantifiers; k++ {
			indices := make([]ast.ASTHandle, arity)
			for j := uint32(0); j < arity; j++ {
				indexSort, y := p.manager.ArrayDomain(sort, j)
				if y != nil {
					return ast.ASTHandle{}, y
				}
				indices[j], y = p.manager.MkVar(p.nextVariable, indexSort)
				if y != nil {
					return ast.ASTHandle{}, y
				}
				p.nextVariable++
			}
			selected, y := p.selectTerm(argument, indices)
			if y != nil {
				return ast.ASTHandle{}, y
			}
			arguments = append(arguments, selected)
			p.representative(argument)
			p.selects[argument] = append(p.selects[argument], selected)
		}
		if !p.enforce {
			arguments = append(arguments, argument)
		}
	}
	return p.createPredicate(source, destination, head, arguments)
}

func (p *ArrayInstantiationPlugin) instantiatePredicate(source, destination *base.RuleSet, application ast.ASTHandle) ([]ast.ASTHandle, error) {
	count, err := p.manager.GetNumArgs(application)
	if err != nil {
		return nil, err
	}
	groups := make([][]ast.ASTHandle, 0)
	for i := uint32(0); i < count; i++ {
		argument, x := p.manager.GetArg(application, i)
		if x != nil {
			return nil, x
		}
		sort, x := p.manager.GetSort(argument)
		if x != nil {
			return nil, x
		}
		array, x := p.isArray(sort)
		if x != nil {
			return nil, x
		}
		if !array {
			groups = append(groups, []ast.ASTHandle{argument})
			continue
		}
		possibilities, x := p.linkedSelects(argument)
		if x != nil {
			return nil, x
		}
		for k := uint32(0); k < p.quantifiers; k++ {
			groups = append(groups, possibilities)
		}
		if !p.enforce {
			groups = append(groups, []ast.ASTHandle{argument})
		}
	}
	chosen := make([]int, len(groups))
	var result []ast.ASTHandle
	for {
		arguments := make([]ast.ASTHandle, len(groups))
		for i := range groups {
			arguments[i] = groups[i][chosen[i]]
		}
		created, x := p.createPredicate(source, destination, application, arguments)
		if x != nil {
			return nil, x
		}
		result = append(result, created)
		position := 0
		for ; position < len(chosen) && chosen[position]+1 >= len(groups[position]); position++ {
		}
		if position == len(chosen) {
			return result, nil
		}
		chosen[position]++
	}
}

func (p *ArrayInstantiationPlugin) transformRule(source, destination *base.RuleSet, rule *base.Rule) (_ *base.Rule, resultErr error) {
	p.selects = make(map[ast.ASTHandle][]ast.ASTHandle)
	p.equivalence = make(map[ast.ASTHandle]ast.ASTHandle)
	p.equivalenceOrder = nil
	p.selectVariables = make(map[ast.ASTHandle]ast.ASTHandle)
	p.selectOrder = nil
	var err error
	p.nextVariable, err = p.maximumVariable(rule)
	if err != nil {
		return nil, err
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	newHead, err := p.createHead(source, destination, head)
	if err != nil {
		return nil, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, err
	}
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return nil, err
	}
	if positive != uninterpreted {
		return nil, fmt.Errorf("muz/transforms: array instantiation does not accept negated predicate tails")
	}
	total, err := rule.TailSize()
	if err != nil {
		return nil, err
	}
	interpreted := make([]ast.ASTHandle, 0, total-uninterpreted)
	for i := uninterpreted; i < total; i++ {
		tail, _, x := rule.Tail(i)
		if x != nil {
			return nil, x
		}
		interpreted = append(interpreted, tail)
		if x = p.retrieveSelects(tail); x != nil {
			return nil, x
		}
	}
	tails := make([]ast.ASTHandle, 0)
	for i := 0; i < uninterpreted; i++ {
		tail, _, x := rule.Tail(i)
		if x != nil {
			return nil, x
		}
		instances, x := p.instantiatePredicate(source, destination, tail)
		if x != nil {
			return nil, x
		}
		tails = append(tails, instances...)
	}
	tails = append(tails, interpreted...)
	for _, selected := range p.selectOrder {
		equality, x := p.manager.MkEq(p.selectVariables[selected], selected)
		if x != nil {
			return nil, x
		}
		tails = append(tails, equality)
	}
	name, err := rule.Name()
	if err != nil {
		return nil, err
	}
	return p.rules.NewRule(newHead, tails, nil, name)
}

func (p *ArrayInstantiationPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	p.oldToNew = make(map[ast.ASTHandle]ast.ASTHandle)
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
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	for i := 0; i < count; i++ {
		rule, x := source.Rule(i)
		if x != nil {
			return nil, x
		}
		replacement, x := p.transformRule(source, result, rule)
		x = errors.Join(x, rule.Close())
		if x != nil {
			return nil, fmt.Errorf("muz/transforms: array instantiation rule %d: %w", i, x)
		}
		x = result.Add(replacement)
		if replacement != nil {
			x = errors.Join(x, replacement.Close())
		}
		if x != nil {
			return nil, x
		}
	}
	if len(p.oldToNew) == 0 {
		return nil, nil
	}
	keep = true
	return result, nil
}

func (p *ArrayInstantiationPlugin) Close() error {
	if p == nil {
		return base.ErrRuleTransformerOwner
	}
	p.closed = true
	p.oldToNew = nil
	p.selects = nil
	p.equivalence = nil
	p.selectVariables = nil
	return nil
}
