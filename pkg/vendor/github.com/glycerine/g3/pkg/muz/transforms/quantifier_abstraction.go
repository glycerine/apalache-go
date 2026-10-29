package transforms

// Source: src/muz/transforms/dl_mk_quantifier_abstraction.cpp/.h retained
// array-predicate abstraction into quantified index/value Horn clauses.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type QuantifierAbstractionPlugin struct {
	manager  *ast.ASTManager
	rules    *base.RuleManager
	priority uint32
	oldToNew map[ast.ASTHandle]ast.ASTHandle
	closed   bool
}

func NewQuantifierAbstractionPlugin(manager *ast.ASTManager, rules *base.RuleManager, priority uint32) *QuantifierAbstractionPlugin {
	return &QuantifierAbstractionPlugin{manager: manager, rules: rules, priority: priority, oldToNew: make(map[ast.ASTHandle]ast.ASTHandle)}
}

func (p *QuantifierAbstractionPlugin) Priority() uint32            { return p.priority }
func (p *QuantifierAbstractionPlugin) CanDestratifyNegation() bool { return false }
func (p *QuantifierAbstractionPlugin) Cancel()                     {}

func (p *QuantifierAbstractionPlugin) FreshPredicates() map[ast.ASTHandle]ast.ASTHandle {
	result := make(map[ast.ASTHandle]ast.ASTHandle, len(p.oldToNew))
	for old, fresh := range p.oldToNew {
		result[old] = fresh
	}
	return result
}

func (p *QuantifierAbstractionPlugin) arraySort(sort ast.ASTHandle) (bool, error) {
	family, err := p.manager.GetFamilyID(sort)
	if err != nil {
		return false, err
	}
	array, err := p.manager.ArrayFamilyID()
	return family == array, err
}

func (p *QuantifierAbstractionPlugin) expandedDomain(sort ast.ASTHandle) ([]ast.ASTHandle, error) {
	var domain []ast.ASTHandle
	for {
		array, err := p.arraySort(sort)
		if err != nil {
			return nil, err
		}
		if !array {
			return append(domain, sort), nil
		}
		arity, err := p.manager.ArrayArity(sort)
		if err != nil {
			return nil, err
		}
		for i := uint32(0); i < arity; i++ {
			index, getErr := p.manager.ArrayDomain(sort, i)
			if getErr != nil {
				return nil, getErr
			}
			domain = append(domain, index)
		}
		sort, err = p.manager.ArrayRange(sort)
		if err != nil {
			return nil, err
		}
	}
}

func (p *QuantifierAbstractionPlugin) declarePredicate(source, destination *base.RuleSet, old ast.ASTHandle) (ast.ASTHandle, bool, error) {
	output, err := source.IsOutputPredicate(old)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if output {
		return old, false, destination.InheritPredicate(source, old, old)
	}
	if fresh := p.oldToNew[old]; !fresh.IsNull() {
		return fresh, true, nil
	}
	arity, err := p.manager.GetArity(old)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	domain := make([]ast.ASTHandle, 0, arity)
	hasArray := false
	for i := uint32(0); i < arity; i++ {
		sort, getErr := p.manager.GetDomain(old, i)
		if getErr != nil {
			return ast.ASTHandle{}, false, getErr
		}
		expanded, getErr := p.expandedDomain(sort)
		if getErr != nil {
			return ast.ASTHandle{}, false, getErr
		}
		hasArray = hasArray || len(expanded) != 1 || expanded[0] != sort
		domain = append(domain, expanded...)
	}
	if !hasArray {
		return old, false, destination.InheritPredicate(source, old, old)
	}
	name, err := p.manager.GetASTName(old)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	rangeSort, err := p.manager.GetRange(old)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	fresh, err := p.manager.MkFuncDecl(name, domain, rangeSort, ast.ASTFuncDeclInfo{})
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if err = p.rules.RegisterPredicate(fresh); err != nil {
		return ast.ASTHandle{}, false, err
	}
	if err = destination.InheritPredicate(source, old, fresh); err != nil {
		return ast.ASTHandle{}, false, err
	}
	p.oldToNew[old] = fresh
	return fresh, true, nil
}

func (p *QuantifierAbstractionPlugin) selectTerm(array ast.ASTHandle, indices []ast.ASTHandle) (ast.ASTHandle, error) {
	arrayFamily, err := p.manager.ArrayFamilyID()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arguments := make([]ast.ASTHandle, 1, len(indices)+1)
	arguments[0] = array
	arguments = append(arguments, indices...)
	return p.manager.MkFamilyApp(arrayFamily, ast.ASTArrayOpSelect, nil, arguments)
}

func (p *QuantifierAbstractionPlugin) makeHead(source, destination *base.RuleSet, application ast.ASTHandle, next uint32) (ast.ASTHandle, error) {
	old, err := p.manager.GetDecl(application)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	fresh, changed, err := p.declarePredicate(source, destination, old)
	if err != nil || !changed {
		return application, err
	}
	count, err := p.manager.GetNumArgs(application)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arguments := make([]ast.ASTHandle, 0)
	for i := uint32(0); i < count; i++ {
		argument, getErr := p.manager.GetArg(application, i)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		sort, getErr := p.manager.GetSort(argument)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		for {
			array, getErr := p.arraySort(sort)
			if getErr != nil {
				return ast.ASTHandle{}, getErr
			}
			if !array {
				break
			}
			arity, getErr := p.manager.ArrayArity(sort)
			if getErr != nil {
				return ast.ASTHandle{}, getErr
			}
			indices := make([]ast.ASTHandle, arity)
			for j := uint32(0); j < arity; j++ {
				indexSort, indexErr := p.manager.ArrayDomain(sort, j)
				if indexErr != nil {
					return ast.ASTHandle{}, indexErr
				}
				indices[j], indexErr = p.manager.MkVar(next, indexSort)
				if indexErr != nil {
					return ast.ASTHandle{}, indexErr
				}
				next++
				arguments = append(arguments, indices[j])
			}
			argument, getErr = p.selectTerm(argument, indices)
			if getErr != nil {
				return ast.ASTHandle{}, getErr
			}
			sort, getErr = p.manager.ArrayRange(sort)
			if getErr != nil {
				return ast.ASTHandle{}, getErr
			}
		}
		arguments = append(arguments, argument)
	}
	return p.manager.MkApp(fresh, arguments)
}

func (p *QuantifierAbstractionPlugin) makeTail(source, destination *base.RuleSet, application ast.ASTHandle) (_ ast.ASTRef, resultErr error) {
	old, err := p.manager.GetDecl(application)
	if err != nil {
		return ast.ASTRef{}, err
	}
	fresh, changed, err := p.declarePredicate(source, destination, old)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if !changed {
		return p.manager.NewRef(application)
	}
	oldArity, err := p.manager.GetArity(old)
	if err != nil {
		return ast.ASTRef{}, err
	}
	newArity, err := p.manager.GetArity(fresh)
	if err != nil || newArity <= oldArity {
		return ast.ASTRef{}, errors.Join(err, ast.ErrASTDomain)
	}
	shift, err := astrewriter.NewVarShifter(p.manager)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, shift.Close()) }()
	shiftedRef, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&shiftedRef)) }()
	if err = shift.ShiftBy(application, newArity-oldArity, shiftedRef); err != nil {
		return ast.ASTRef{}, err
	}
	shifted, err := p.manager.RefGet(shiftedRef)
	if err != nil {
		return ast.ASTRef{}, err
	}
	count, err := p.manager.GetNumArgs(shifted)
	if err != nil {
		return ast.ASTRef{}, err
	}
	arguments := make([]ast.ASTHandle, 0)
	var boundSorts []ast.ASTHandle
	var names []usymbol.Symbol
	var triggers []ast.ASTHandle
	for i := uint32(0); i < count; i++ {
		argument, getErr := p.manager.GetArg(shifted, i)
		if getErr != nil {
			return ast.ASTRef{}, getErr
		}
		sort, getErr := p.manager.GetSort(argument)
		if getErr != nil {
			return ast.ASTRef{}, getErr
		}
		arrayArgument := false
		for {
			array, getErr := p.arraySort(sort)
			if getErr != nil {
				return ast.ASTRef{}, getErr
			}
			if !array {
				break
			}
			arrayArgument = true
			arity, getErr := p.manager.ArrayArity(sort)
			if getErr != nil {
				return ast.ASTRef{}, getErr
			}
			indices := make([]ast.ASTHandle, arity)
			for j := uint32(0); j < arity; j++ {
				indexSort, indexErr := p.manager.ArrayDomain(sort, j)
				if indexErr != nil {
					return ast.ASTRef{}, indexErr
				}
				indices[j], indexErr = p.manager.MkVar(uint32(len(boundSorts)), indexSort)
				if indexErr != nil {
					return ast.ASTRef{}, indexErr
				}
				boundSorts = append(boundSorts, indexSort)
				names = append(names, usymbol.Numerical(uint32(len(names))))
				arguments = append(arguments, indices[j])
			}
			argument, getErr = p.selectTerm(argument, indices)
			if getErr != nil {
				return ast.ASTRef{}, getErr
			}
			sort, getErr = p.manager.ArrayRange(sort)
			if getErr != nil {
				return ast.ASTRef{}, getErr
			}
		}
		if arrayArgument {
			triggers = append(triggers, argument)
		}
		arguments = append(arguments, argument)
	}
	body, err := p.manager.MkApp(fresh, arguments)
	if err != nil {
		return ast.ASTRef{}, err
	}
	annotation, err := p.manager.MkPattern(triggers)
	if err != nil {
		return ast.ASTRef{}, err
	}
	quantified, err := p.manager.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: boundSorts, Names: names, Body: body, Weight: 1,
		Patterns: []ast.ASTHandle{annotation},
	})
	if err != nil {
		return ast.ASTRef{}, err
	}
	truth, err := p.manager.MkTrue()
	if err != nil {
		return ast.ASTRef{}, err
	}
	wrapper, err := p.manager.MkEq(quantified, truth)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return p.manager.NewRef(wrapper)
}

func (p *QuantifierAbstractionPlugin) transformRule(source, destination *base.RuleSet, rule *base.Rule) (_ *base.Rule, resultErr error) {
	variables, err := ruleVariableCount(p.manager, rule)
	if err != nil {
		return nil, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, err
	}
	total, err := rule.TailSize()
	if err != nil {
		return nil, err
	}
	tails := make([]ast.ASTHandle, 0, total)
	negated := make([]bool, 0, total)
	owned := make([]ast.ASTRef, 0, uninterpreted)
	defer func() {
		for i := len(owned) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&owned[i]))
		}
	}()
	for i := 0; i < uninterpreted; i++ {
		tail, negative, getErr := rule.Tail(i)
		if getErr != nil {
			return nil, getErr
		}
		transformed, getErr := p.makeTail(source, destination, tail)
		if getErr != nil {
			return nil, getErr
		}
		owned = append(owned, transformed)
		tail, getErr = p.manager.RefGet(transformed)
		if getErr != nil {
			return nil, getErr
		}
		tails, negated = append(tails, tail), append(negated, negative)
	}
	for i := uninterpreted; i < total; i++ {
		tail, negative, getErr := rule.Tail(i)
		if getErr != nil {
			return nil, getErr
		}
		tails, negated = append(tails, tail), append(negated, negative)
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	head, err = p.makeHead(source, destination, head, variables)
	if err != nil {
		return nil, err
	}
	used, err := ast.NewASTUsedVars(p.manager)
	if err != nil {
		return nil, err
	}
	if err = used.Process(head); err != nil {
		return nil, err
	}
	for _, tail := range tails {
		if err = used.Process(tail); err != nil {
			return nil, err
		}
	}
	next := used.MaxFoundVarIndexPlusOne()
	declaration, err := p.manager.GetDecl(head)
	if err != nil {
		return nil, err
	}
	arguments, err := p.manager.GetNumArgs(head)
	if err != nil {
		return nil, err
	}
	headArguments := make([]ast.ASTHandle, arguments)
	headChanged := false
	for i := uint32(0); i < arguments; i++ {
		argument, getErr := p.manager.GetArg(head, i)
		if getErr != nil {
			return nil, getErr
		}
		kind, getErr := p.manager.GetKind(argument)
		if getErr != nil {
			return nil, getErr
		}
		value, getErr := p.manager.IsValue(argument)
		if getErr != nil {
			return nil, getErr
		}
		if kind == ast.ASTVar || value {
			headArguments[i] = argument
			continue
		}
		sort, getErr := p.manager.GetSort(argument)
		if getErr != nil {
			return nil, getErr
		}
		variable, getErr := p.manager.MkVar(next, sort)
		if getErr != nil {
			return nil, getErr
		}
		next++
		equality, getErr := p.manager.MkEq(variable, argument)
		if getErr != nil {
			return nil, getErr
		}
		headArguments[i] = variable
		tails, negated = append(tails, equality), append(negated, false)
		headChanged = true
	}
	if headChanged {
		head, err = p.manager.MkApp(declaration, headArguments)
		if err != nil {
			return nil, err
		}
	}
	name, err := rule.Name()
	if err != nil {
		return nil, err
	}
	return p.rules.NewRule(head, tails, negated, name)
}

func (p *QuantifierAbstractionPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	for i := 0; i < count; i++ {
		rule, getErr := source.Rule(i)
		if getErr != nil {
			return nil, getErr
		}
		positive, getErr := rule.PositiveTailSize()
		if getErr == nil {
			var uninterpreted int
			uninterpreted, getErr = rule.UninterpretedTailSize()
			if getErr == nil && positive != uninterpreted {
				if closeErr := rule.Close(); closeErr != nil {
					return nil, closeErr
				}
				return nil, nil
			}
		}
		getErr = errors.Join(getErr, rule.Close())
		if getErr != nil {
			return nil, getErr
		}
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
	for i := 0; i < count; i++ {
		rule, getErr := source.Rule(i)
		if getErr != nil {
			return nil, getErr
		}
		replacement, transformErr := p.transformRule(source, result, rule)
		if transformErr == nil {
			transformErr = result.Add(replacement)
		}
		transformErr = errors.Join(transformErr, rule.Close())
		if replacement != nil {
			transformErr = errors.Join(transformErr, replacement.Close())
		}
		if transformErr != nil {
			return nil, transformErr
		}
	}
	if len(p.oldToNew) == 0 {
		return nil, nil
	}
	keep = true
	return result, nil
}

func (p *QuantifierAbstractionPlugin) Close() error {
	if p == nil {
		return base.ErrRuleTransformerOwner
	}
	p.closed = true
	p.oldToNew = nil
	return nil
}
