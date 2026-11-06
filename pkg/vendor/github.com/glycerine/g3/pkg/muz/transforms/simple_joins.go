package transforms

// Source: src/muz/rel/dl_mk_simple_joins.cpp and .h retained rule-shape
// normalization to at most two positive predicate tails.

import (
	"errors"
	"fmt"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type SimpleJoinsPlugin struct {
	manager   *ast.ASTManager
	rules     *base.RuleManager
	relations *rel.RelationManager
	priority  uint32
	fresh     []ast.ASTHandle
	origins   map[ast.ASTHandle]ast.ASTHandle
	pins      []ast.ASTRef
	closed    bool
	closing   bool
}

type sharedJoinKey struct {
	pair    normalizedPairKey
	outputs string
}

type sharedJoinDefinition struct {
	declaration ast.ASTHandle
	outputs     []uint32
}

func NewSimpleJoinsPlugin(manager *ast.ASTManager, rules *base.RuleManager, priority uint32, relations ...*rel.RelationManager) *SimpleJoinsPlugin {
	if priority == 0 {
		priority = 1000
	}
	plugin := &SimpleJoinsPlugin{manager: manager, rules: rules, priority: priority}
	if len(relations) > 0 {
		plugin.relations = relations[0]
	}
	return plugin
}

func (p *SimpleJoinsPlugin) Priority() uint32            { return p.priority }
func (p *SimpleJoinsPlugin) CanDestratifyNegation() bool { return false }
func (p *SimpleJoinsPlugin) Cancel()                     {}

func (p *SimpleJoinsPlugin) FreshPredicates() []ast.ASTHandle {
	if p == nil || p.closed || p.closing {
		return nil
	}
	return append([]ast.ASTHandle(nil), p.fresh...)
}

// FreshPredicateOrigin returns the parent head passed to
// context::mk_fresh_head_predicate by the source join planner. A shared helper
// keeps the last registered parent's origin selected when it was created.
// Both declarations are borrowed from the rule manager.
func (p *SimpleJoinsPlugin) FreshPredicateOrigin(predicate ast.ASTHandle) (ast.ASTHandle, bool) {
	if p == nil || p.closed || p.closing {
		return ast.ASTHandle{}, false
	}
	origin, found := p.origins[predicate]
	return origin, found
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

func (p *SimpleJoinsPlugin) auxiliary(pair normalizedPair, outside []ast.ASTHandle, prefix usymbol.Symbol, parent *base.Rule, shared map[sharedJoinKey]sharedJoinDefinition, globalOutputs map[normalizedPairKey][]uint32) (ast.ASTHandle, *base.Rule, error) {
	var err error
	live := make(map[uint32]ast.ASTHandle)
	for _, expression := range outside {
		if err := p.variables(expression, live); err != nil {
			return ast.ASTHandle{}, nil, err
		}
	}
	outputs := make([]uint32, 0)
	for variable := range pair.actualToCanonical {
		if _, found := live[variable]; found {
			outputs = append(outputs, pair.actualToCanonical[variable])
		}
	}
	if union, found := globalOutputs[pair.key]; found {
		outputs = append(outputs[:0], union...)
	}
	sort.Slice(outputs, func(i, j int) bool { return outputs[i] < outputs[j] })
	key := sharedJoinKey{pair: pair.key, outputs: fmt.Sprint(outputs)}
	entry, found := shared[key]
	var definition *base.Rule
	if !found {
		var origin ast.ASTHandle
		if parent != nil {
			origin, err = parent.HeadDeclaration()
			if err != nil {
				return ast.ASTHandle{}, nil, err
			}
		}
		domain := make([]ast.ASTHandle, len(outputs))
		canonicalArguments := make([]ast.ASTHandle, len(outputs))
		for i, output := range outputs {
			domain[i] = pair.canonicalSorts[output]
			canonicalArguments[i], err = p.manager.MkVar(output, domain[i])
			if err != nil {
				return ast.ASTHandle{}, nil, err
			}
		}
		boolean, err := p.manager.MkBoolSort()
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		suffix, err := p.manager.Symbols.Intern("split")
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		declaration, err := p.manager.MkFreshFuncDecl(prefix, suffix, domain, boolean)
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		if err = p.rules.RegisterPredicate(declaration); err != nil {
			return ast.ASTHandle{}, nil, err
		}
		p.fresh = append(p.fresh, declaration)
		if !origin.IsNull() {
			if p.origins == nil {
				p.origins = make(map[ast.ASTHandle]ast.ASTHandle)
			}
			p.origins[declaration] = origin
		}
		canonicalHead, err := p.manager.MkApp(declaration, canonicalArguments)
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		definition, err = p.rules.NewRule(canonicalHead, pair.terms[:], nil, "")
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		if parent != nil {
			if err = definition.SetAccountingParentRule(parent); err != nil {
				return ast.ASTHandle{}, nil, errors.Join(err, definition.Close())
			}
		}
		if err = p.attachAssertedProof(definition); err != nil {
			return ast.ASTHandle{}, nil, errors.Join(err, definition.Close())
		}
		entry = sharedJoinDefinition{declaration: declaration, outputs: append([]uint32(nil), outputs...)}
		shared[key] = entry
	}
	arguments := make([]ast.ASTHandle, len(entry.outputs))
	for i, output := range entry.outputs {
		actual := pair.canonicalToActual[output]
		arguments[i], err = p.manager.MkVar(actual, pair.canonicalSorts[output])
		if err != nil {
			return ast.ASTHandle{}, definition, err
		}
	}
	head, err := p.manager.MkApp(entry.declaration, arguments)
	if err != nil {
		return ast.ASTHandle{}, definition, err
	}
	pin, err := p.manager.NewRef(head)
	if err != nil {
		return ast.ASTHandle{}, definition, err
	}
	p.pins = append(p.pins, pin)
	return head, definition, nil
}

func (p *SimpleJoinsPlugin) attachAssertedProof(rule *base.Rule) (resultErr error) {
	enabled, err := p.manager.ProofsEnabled()
	if err != nil || !enabled {
		return err
	}
	formula, err := rule.ToFormula()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&formula)) }()
	formulaHandle, err := p.manager.RefGet(formula)
	if err != nil {
		return err
	}
	proof, err := p.manager.MkProofAsserted(formulaHandle)
	if err != nil {
		return err
	}
	return rule.SetProof(proof)
}

func (p *SimpleJoinsPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.closing || p.manager == nil || p.rules == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	states := make([]*simpleJoinRuleState, 0, count)
	defer func() {
		for _, state := range states {
			resultErr = errors.Join(resultErr, state.rule.Close())
		}
	}()
	for index := 0; index < count; index++ {
		rule, x := source.Rule(index)
		if x != nil {
			return nil, x
		}
		state, x := newSimpleJoinState(p, rule)
		if x != nil {
			return nil, errors.Join(x, rule.Close())
		}
		states = append(states, state)
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
	if err = result.InheritPredicates(source); err != nil {
		return nil, err
	}
	strata, err := base.NewRuleSet(p.rules)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, strata.Close()) }()
	if err = strata.AddRules(source); err != nil {
		return nil, err
	}
	if _, err = strata.Finalize(); err != nil {
		return nil, err
	}
	var introduced []*base.Rule
	shared := make(map[sharedJoinKey]sharedJoinDefinition)
	costs := make(map[normalizedPairKey]float32)
	stratified := make(map[normalizedPairKey]bool)
	outputHistory := make(map[normalizedPairKey]map[uint32]struct{})
	registrationHistory := make(map[normalizedPairKey][]*simpleJoinRuleState)
	traceCounts := make(map[normalizedPairKey]map[*simpleJoinRuleState]int)
	defer func() {
		for _, definition := range introduced {
			if definition != nil {
				resultErr = errors.Join(resultErr, definition.Close())
			}
		}
	}()
	changed := false
	for {
		groups, x := p.activeJoinGroups(states, strata, costs, stratified, outputHistory, registrationHistory, traceCounts)
		if x != nil {
			return nil, x
		}
		selected := chooseSimpleJoinGroup(groups)
		if selected == nil {
			break
		}
		outputs := groupOutputs(selected, outputHistory)
		if x = p.applySimpleJoinGroup(selected, outputs, shared, &introduced, registrationHistory, traceCounts); x != nil {
			return nil, x
		}
		changed = true
	}
	for _, state := range states {
		if !state.modified {
			if err = result.Add(state.rule); err != nil {
				return nil, err
			}
			continue
		}
		replacement, x := p.rules.NewRuleWithSymbol(state.head, state.tails, state.negated, state.name)
		if x == nil {
			x = replacement.SetAccountingParentRule(state.rule)
		}
		if x == nil {
			x = p.rules.MkRuleRewriteProof(state.rule, replacement)
		}
		if x == nil {
			x = result.Add(replacement)
		}
		if replacement != nil {
			x = errors.Join(x, replacement.Close())
		}
		if x != nil {
			return nil, x
		}
	}
	for i := len(introduced) - 1; i >= 0; i-- {
		definition := introduced[i]
		x := result.Add(definition)
		x = errors.Join(x, definition.Close())
		introduced[i] = nil
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
	p.closing = true
	for len(p.pins) != 0 {
		last := len(p.pins) - 1
		err := p.manager.DelRef(&p.pins[last])
		if p.pins[last] == (ast.ASTRef{}) {
			p.pins = p.pins[:last]
		}
		if err != nil {
			return err
		}
	}
	p.pins = nil
	p.fresh = nil
	p.origins = nil
	p.closed = true
	return nil
}
