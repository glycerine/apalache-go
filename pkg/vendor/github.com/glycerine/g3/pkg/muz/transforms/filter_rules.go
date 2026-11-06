package transforms

// Source: src/muz/transforms/dl_mk_filter_rules.cpp/.h and the retained
// mk_new_rule_tail specialization from src/muz/base/dl_util.cpp/.h.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

type filterRuleCacheEntry struct {
	predicate   ast.ASTHandle
	arguments   []ast.ASTHandle
	declaration ast.ASTHandle
	origin      ast.ASTHandle
	pin         ast.ASTRef
}

type FilterRulesPlugin struct {
	manager  *ast.ASTManager
	rules    *base.RuleManager
	priority uint32
	cache    []filterRuleCacheEntry
	closed   bool
	closing  bool
}

func NewFilterRulesPlugin(manager *ast.ASTManager, rules *base.RuleManager, priority uint32) *FilterRulesPlugin {
	if priority == 0 {
		priority = 2000
	}
	return &FilterRulesPlugin{manager: manager, rules: rules, priority: priority}
}

func (p *FilterRulesPlugin) Priority() uint32            { return p.priority }
func (p *FilterRulesPlugin) CanDestratifyNegation() bool { return false }
func (p *FilterRulesPlugin) Cancel()                     {}

// FreshPredicates returns borrowed filter declarations in creation order.
func (p *FilterRulesPlugin) FreshPredicates() []ast.ASTHandle {
	if p == nil || p.closed || p.closing {
		return nil
	}
	result := make([]ast.ASTHandle, len(p.cache))
	for i := range p.cache {
		result[i] = p.cache[i].declaration
	}
	return result
}

// FreshPredicateOrigin returns the filtered tail declaration passed to
// context::mk_fresh_head_predicate, whose requested relation kind the filter
// inherits. Both declarations are borrowed from the rule manager.
func (p *FilterRulesPlugin) FreshPredicateOrigin(predicate ast.ASTHandle) (ast.ASTHandle, bool) {
	if p == nil || p.closed || p.closing {
		return ast.ASTHandle{}, false
	}
	for _, entry := range p.cache {
		if entry.declaration == predicate {
			return entry.origin, true
		}
	}
	return ast.ASTHandle{}, false
}

func (p *FilterRulesPlugin) resetCache() error {
	for len(p.cache) != 0 {
		last := len(p.cache) - 1
		err := p.manager.DelRef(&p.cache[last].pin)
		if p.cache[last].pin == (ast.ASTRef{}) {
			p.cache[last] = filterRuleCacheEntry{}
			p.cache = p.cache[:last]
		}
		if err != nil {
			return err
		}
	}
	p.cache = nil
	return nil
}

func (p *FilterRulesPlugin) isCandidate(application ast.ASTHandle) (bool, error) {
	count, err := p.manager.GetNumArgs(application)
	if err != nil {
		return false, err
	}
	used := make(map[uint32]bool)
	for index := uint32(0); index < count; index++ {
		argument, getErr := p.manager.GetArg(application, index)
		if getErr != nil {
			return false, getErr
		}
		value, getErr := p.manager.IsValue(argument)
		if getErr != nil {
			return false, getErr
		}
		if value {
			return true, nil
		}
		kind, getErr := p.manager.GetKind(argument)
		if getErr != nil {
			return false, getErr
		}
		if kind != ast.ASTVar {
			return false, fmt.Errorf("muz/transforms: non-value predicate argument is not a variable")
		}
		variable, getErr := p.manager.GetVarIndex(argument)
		if getErr != nil {
			return false, getErr
		}
		if used[variable] {
			return true, nil
		}
		used[variable] = true
	}
	return false, nil
}

// nonLocalVariables mirrors collect_rule_vars_ex: every occurrence of the
// selected application handle is excluded, not merely its selected index.
func (p *FilterRulesPlugin) nonLocalVariables(rule *base.Rule, selected ast.ASTHandle) (map[uint32]bool, error) {
	used, err := ast.NewASTUsedVars(p.manager)
	if err != nil {
		return nil, err
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	if err = used.Process(head); err != nil {
		return nil, err
	}
	count, err := rule.TailSize()
	if err != nil {
		return nil, err
	}
	for index := 0; index < count; index++ {
		tail, _, getErr := rule.Tail(index)
		if getErr != nil {
			return nil, getErr
		}
		if tail == selected {
			continue
		}
		if err = used.Process(tail); err != nil {
			return nil, err
		}
	}
	result := make(map[uint32]bool)
	for index := uint32(0); index < used.MaxFoundVarIndexPlusOne(); index++ {
		if !used.Contains(index).IsNull() {
			result[index] = true
		}
	}
	return result, nil
}

func sameFilterArguments(left, right []ast.ASTHandle) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func (p *FilterRulesPlugin) makeFilter(application ast.ASTHandle, nonLocal map[uint32]bool, result *base.RuleSet, parent *base.Rule) (_ ast.ASTHandle, resultErr error) {
	count, err := p.manager.GetNumArgs(application)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	mapped := make(map[uint32]ast.ASTHandle)
	next := uint32(0)
	newArguments := make([]ast.ASTHandle, 0, count)
	filterArguments := make([]ast.ASTHandle, 0, count)
	domain := make([]ast.ASTHandle, 0, count)
	for index := uint32(0); index < count; index++ {
		argument, getErr := p.manager.GetArg(application, index)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		value, getErr := p.manager.IsValue(argument)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		if value {
			newArguments = append(newArguments, argument)
			continue
		}
		variable, getErr := p.manager.GetVarIndex(argument)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		replacement, found := mapped[variable]
		if !found {
			sort, sortErr := p.manager.GetSort(argument)
			if sortErr != nil {
				return ast.ASTHandle{}, sortErr
			}
			replacement, getErr = p.manager.MkVar(next, sort)
			if getErr != nil {
				return ast.ASTHandle{}, getErr
			}
			next++
			mapped[variable] = replacement
			if nonLocal[variable] {
				domain = append(domain, sort)
				filterArguments = append(filterArguments, replacement)
			}
		}
		newArguments = append(newArguments, replacement)
	}
	originalDeclaration, err := p.manager.GetDecl(application)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	normalized, err := p.manager.MkApp(originalDeclaration, newArguments)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	normalizedPin, err := p.manager.NewRef(normalized)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	keepNormalizedPin := false
	defer func() {
		if !keepNormalizedPin {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&normalizedPin))
		}
	}()
	for index := range p.cache {
		entry := &p.cache[index]
		if entry.predicate == normalized && sameFilterArguments(entry.arguments, filterArguments) {
			return entry.declaration, nil
		}
	}
	prefix, err := p.manager.GetASTName(originalDeclaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	suffix, err := p.manager.Symbols.Intern("filter")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	boolean, err := p.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	filterDeclaration, err := p.manager.MkFreshFuncDecl(prefix, suffix, domain, boolean)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = p.rules.RegisterPredicate(filterDeclaration); err != nil {
		return ast.ASTHandle{}, err
	}
	filterHead, err := p.manager.MkApp(filterDeclaration, filterArguments)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	headPin, err := p.manager.NewRef(filterHead)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&headPin)) }()
	definition, err := p.rules.NewRule(filterHead, []ast.ASTHandle{normalized}, nil, "")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, definition.Close()) }()
	if err = definition.SetAccountingParentRule(parent); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = result.Add(definition); err != nil {
		return ast.ASTHandle{}, err
	}
	p.cache = append(p.cache, filterRuleCacheEntry{
		predicate: normalized, arguments: append([]ast.ASTHandle(nil), filterArguments...),
		declaration: filterDeclaration, origin: originalDeclaration, pin: normalizedPin,
	})
	keepNormalizedPin = true
	return filterDeclaration, nil
}

func removeDuplicateFilterTails(tails []ast.ASTHandle, negated []bool) ([]ast.ASTHandle, []bool) {
	positive := make(map[ast.ASTHandle]bool)
	negative := make(map[ast.ASTHandle]bool)
	index := 0
	for index < len(tails) {
		seen := positive
		if negated[index] {
			seen = negative
		}
		if seen[tails[index]] {
			last := len(tails) - 1
			if index != last {
				tails[index] = tails[last]
				negated[index] = negated[last]
			}
			tails = tails[:last]
			negated = negated[:last]
			continue
		}
		seen[tails[index]] = true
		index++
	}
	return tails, negated
}

func (p *FilterRulesPlugin) rewrite(rule *base.Rule, result *base.RuleSet) (changed bool, resultErr error) {
	head, err := rule.Head()
	if err != nil {
		return false, err
	}
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return false, err
	}
	total, err := rule.TailSize()
	if err != nil {
		return false, err
	}
	tails := make([]ast.ASTHandle, 0, total)
	negated := make([]bool, 0, total)
	for index := 0; index < total; index++ {
		tail, isNegated, getErr := rule.Tail(index)
		if getErr != nil {
			return false, getErr
		}
		candidate := false
		if index < positive {
			candidate, getErr = p.isCandidate(tail)
			if getErr != nil {
				return false, getErr
			}
		}
		if candidate {
			nonLocal, collectErr := p.nonLocalVariables(rule, tail)
			if collectErr != nil {
				return false, collectErr
			}
			filterDeclaration, makeErr := p.makeFilter(tail, nonLocal, result, rule)
			if makeErr != nil {
				return false, makeErr
			}
			arguments := make([]ast.ASTHandle, 0)
			used := make(map[uint32]bool)
			count, makeErr := p.manager.GetNumArgs(tail)
			for argumentIndex := uint32(0); makeErr == nil && argumentIndex < count; argumentIndex++ {
				var argument ast.ASTHandle
				argument, makeErr = p.manager.GetArg(tail, argumentIndex)
				if makeErr != nil {
					break
				}
				kind, kindErr := p.manager.GetKind(argument)
				makeErr = kindErr
				if kindErr != nil || kind != ast.ASTVar {
					continue
				}
				variable, variableErr := p.manager.GetVarIndex(argument)
				makeErr = variableErr
				if variableErr == nil && nonLocal[variable] && !used[variable] {
					arguments = append(arguments, argument)
					used[variable] = true
				}
			}
			if makeErr != nil {
				return false, makeErr
			}
			tail, getErr = p.manager.MkApp(filterDeclaration, arguments)
			if getErr != nil {
				return false, getErr
			}
			isNegated = false
			changed = true
		}
		tails = append(tails, tail)
		negated = append(negated, isNegated)
	}
	if !changed {
		return false, result.Add(rule)
	}
	tails, negated = removeDuplicateFilterTails(tails, negated)
	name, err := rule.NameSymbol()
	if err != nil {
		return false, err
	}
	replacement, err := p.rules.NewRuleWithSymbol(head, tails, negated, name)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, replacement.Close()) }()
	if err = replacement.SetAccountingParentRule(rule); err != nil {
		return false, err
	}
	return true, result.Add(replacement)
}

func (p *FilterRulesPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.closing || p.manager == nil || p.rules == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	if err := p.resetCache(); err != nil {
		return nil, err
	}
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
	modified := false
	for index := 0; index < count; index++ {
		rule, getErr := source.Rule(index)
		if getErr != nil {
			return nil, getErr
		}
		changed, rewriteErr := p.rewrite(rule, result)
		rewriteErr = errors.Join(rewriteErr, rule.Close())
		if rewriteErr != nil {
			return nil, rewriteErr
		}
		modified = modified || changed
	}
	if !modified {
		return nil, nil
	}
	if err = result.InheritPredicates(source); err != nil {
		return nil, err
	}
	keep = true
	return result, nil
}

func (p *FilterRulesPlugin) Close() error {
	if p == nil || p.closed {
		return nil
	}
	p.closing = true
	if err := p.resetCache(); err != nil {
		return err
	}
	p.closed = true
	return nil
}
