package transforms

// Source: src/muz/transforms/dl_mk_rule_inliner.cpp/.h retained planning,
// offset-unification, resolution, and eliminated-predicate rule rewriting.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/substitution"
	"github.com/glycerine/g3/pkg/muz/base"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type RuleInlinerPlugin struct {
	manager    *ast.ASTManager
	rules      *base.RuleManager
	protected  map[ast.ASTHandle]bool
	priority   uint32
	eager      bool
	linear     bool
	params     *uparams.ParamsContext
	globals    *uparams.GlobalParamsService
	simplifier *InterpTailSimplifierPlugin
	closed     bool
}

// NewRuleInlinerPluginWithServices enables the source post-resolution
// interpreted-tail simplifier using the owning parameter services.
func NewRuleInlinerPluginWithServices(manager *ast.ASTManager, rules *base.RuleManager, protected []ast.ASTHandle, eager, linear bool, params *uparams.ParamsContext, globals *uparams.GlobalParamsService, priority uint32) *RuleInlinerPlugin {
	p := NewRuleInlinerPluginWithModes(manager, rules, protected, eager, linear, priority)
	p.params, p.globals = params, globals
	return p
}

func NewRuleInlinerPlugin(manager *ast.ASTManager, rules *base.RuleManager, protected []ast.ASTHandle, priority uint32) *RuleInlinerPlugin {
	return NewRuleInlinerPluginWithModes(manager, rules, protected, true, true, priority)
}

// NewRuleInlinerPluginWithModes constructs the source parameterized eager and
// straight-line inlining stages.
func NewRuleInlinerPluginWithModes(manager *ast.ASTManager, rules *base.RuleManager, protected []ast.ASTHandle, eager, linear bool, priority uint32) *RuleInlinerPlugin {
	if priority == 0 {
		priority = 35000
	}
	p := &RuleInlinerPlugin{manager: manager, rules: rules, protected: make(map[ast.ASTHandle]bool), priority: priority, eager: eager, linear: linear}
	for _, declaration := range protected {
		p.protected[declaration] = true
	}
	return p
}

func (p *RuleInlinerPlugin) Priority() uint32            { return p.priority }
func (p *RuleInlinerPlugin) CanDestratifyNegation() bool { return false }
func (p *RuleInlinerPlugin) Cancel()                     {}

func closeRules(rules []*base.Rule) (result error) {
	for i := len(rules) - 1; i >= 0; i-- {
		result = errors.Join(result, rules[i].Close())
	}
	return result
}

func ruleVariableCount(manager *ast.ASTManager, rule *base.Rule) (uint32, error) {
	used, err := ast.NewASTUsedVars(manager)
	if err != nil {
		return 0, err
	}
	head, err := rule.Head()
	if err != nil {
		return 0, err
	}
	used.Reset()
	if err = used.Process(head); err != nil {
		return 0, err
	}
	tails, err := rule.TailSize()
	if err != nil {
		return 0, err
	}
	for index := 0; index < tails; index++ {
		tail, _, getErr := rule.Tail(index)
		if getErr != nil {
			return 0, getErr
		}
		if err = used.Process(tail); err != nil {
			return 0, err
		}
	}
	return used.MaxFoundVarIndexPlusOne(), nil
}

func (p *RuleInlinerPlugin) resolve(target *base.Rule, tailIndex int, source *base.Rule) (_ *base.Rule, resultErr error) {
	tail, negated, err := target.Tail(tailIndex)
	if err != nil || negated {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return nil, err
	}
	sourceHead, err := source.Head()
	if err != nil {
		return nil, err
	}
	targetVariables, err := ruleVariableCount(p.manager, target)
	if err != nil {
		return nil, err
	}
	sourceVariables, err := ruleVariableCount(p.manager, source)
	if err != nil {
		return nil, err
	}
	variables := targetVariables
	if sourceVariables > variables {
		variables = sourceVariables
	}
	subst, err := substitution.New(p.manager)
	if err != nil {
		return nil, err
	}
	unifier, err := substitution.NewUnifier(p.manager)
	if err != nil {
		return nil, errors.Join(err, subst.Close())
	}
	defer func() { resultErr = errors.Join(resultErr, unifier.Close(), subst.Close()) }()
	if err = subst.Reserve(2, variables); err != nil {
		return nil, err
	}
	unified, err := unifier.UnifyPair(tail, sourceHead, subst, true)
	if err != nil {
		return nil, fmt.Errorf("muz/transforms: unify inline tail and head: %w", err)
	}
	if !unified {
		return nil, nil
	}
	deltas := []uint32{0, variables}
	var references []ast.ASTRef
	defer func() { resultErr = errors.Join(resultErr, releaseTransformRefs(p.manager, references)) }()
	apply := func(expression ast.ASTHandle, offset uint32) (ast.ASTHandle, error) {
		ref, applyErr := p.manager.NewRef(ast.ASTHandle{})
		if applyErr != nil {
			return ast.ASTHandle{}, applyErr
		}
		references = append(references, ref)
		if applyErr = subst.Apply(2, deltas, substitution.NewExprOffset(expression, offset), ref); applyErr != nil {
			return ast.ASTHandle{}, applyErr
		}
		return p.manager.RefGet(ref)
	}
	targetHead, err := target.Head()
	if err != nil {
		return nil, err
	}
	newHead, err := apply(targetHead, 0)
	if err != nil {
		return nil, fmt.Errorf("muz/transforms: apply inline target head: %w", err)
	}
	var tails []ast.ASTHandle
	var negatives []bool
	appendRule := func(rule *base.Rule, offset uint32, skip int) error {
		count, appendErr := rule.TailSize()
		if appendErr != nil {
			return appendErr
		}
		for index := 0; index < count; index++ {
			if index == skip {
				continue
			}
			expression, negative, getErr := rule.Tail(index)
			if getErr != nil {
				return getErr
			}
			updated, getErr := apply(expression, offset)
			if getErr != nil {
				return getErr
			}
			duplicate := false
			for prior := range tails {
				if tails[prior] == updated && negatives[prior] == negative {
					duplicate = true
					break
				}
			}
			if !duplicate {
				tails = append(tails, updated)
				negatives = append(negatives, negative)
			}
		}
		return nil
	}
	if err = appendRule(target, 0, tailIndex); err != nil {
		return nil, fmt.Errorf("muz/transforms: apply inline target tail: %w", err)
	}
	if err = appendRule(source, 1, -1); err != nil {
		return nil, fmt.Errorf("muz/transforms: apply inline source tail: %w", err)
	}
	targetName, err := target.Name()
	if err != nil {
		return nil, err
	}
	sourceName, err := source.Name()
	if err != nil {
		return nil, err
	}
	resolved, err := p.rules.NewRule(newHead, tails, negatives, targetName+";"+sourceName)
	if err != nil {
		return nil, fmt.Errorf("muz/transforms: construct resolved rule: %w", err)
	}
	if p.params == nil || p.globals == nil {
		return resolved, nil
	}
	if p.simplifier == nil {
		p.simplifier, err = NewInterpTailSimplifierPlugin(p.manager, p.rules, p.params, p.globals, p.priority)
		if err != nil {
			return nil, errors.Join(err, resolved.Close())
		}
	}
	replacement, dropped, changed, err := p.simplifier.transformRule(resolved)
	if err != nil {
		return nil, errors.Join(err, resolved.Close())
	}
	if dropped {
		return nil, resolved.Close()
	}
	if !changed || replacement == nil {
		return resolved, nil
	}
	if err = resolved.Close(); err != nil {
		return nil, errors.Join(err, replacement.Close())
	}
	return replacement, nil
}

func releaseTransformRefs(manager *ast.ASTManager, refs []ast.ASTRef) (result error) {
	for i := len(refs) - 1; i >= 0; i-- {
		result = errors.Join(result, manager.DelRef(&refs[i]))
	}
	return result
}

func inlinerComponents(order []ast.ASTHandle, candidate, forbidden map[ast.ASTHandle]bool, edges map[ast.ASTHandle]map[ast.ASTHandle]bool) [][]ast.ASTHandle {
	index := 0
	indices := make(map[ast.ASTHandle]int)
	lowlink := make(map[ast.ASTHandle]int)
	onStack := make(map[ast.ASTHandle]bool)
	var stack []ast.ASTHandle
	var components [][]ast.ASTHandle
	var visit func(ast.ASTHandle)
	visit = func(node ast.ASTHandle) {
		index++
		indices[node] = index
		lowlink[node] = index
		stack = append(stack, node)
		onStack[node] = true
		for next := range edges[node] {
			if !candidate[next] || forbidden[next] {
				continue
			}
			if indices[next] == 0 {
				visit(next)
				if lowlink[next] < lowlink[node] {
					lowlink[node] = lowlink[next]
				}
			} else if onStack[next] && indices[next] < lowlink[node] {
				lowlink[node] = indices[next]
			}
		}
		if lowlink[node] != indices[node] {
			return
		}
		var component []ast.ASTHandle
		for {
			last := len(stack) - 1
			member := stack[last]
			stack = stack[:last]
			onStack[member] = false
			component = append(component, member)
			if member == node {
				break
			}
		}
		components = append(components, component)
	}
	for _, declaration := range order {
		if candidate[declaration] && !forbidden[declaration] && indices[declaration] == 0 {
			visit(declaration)
		}
	}
	return components
}

func breakInliningCycles(order []ast.ASTHandle, candidate map[ast.ASTHandle]bool, edges map[ast.ASTHandle]map[ast.ASTHandle]bool) map[ast.ASTHandle]bool {
	forbidden := make(map[ast.ASTHandle]bool)
	// A singleton self-cycle remains unchanged by resolution, so attempting to
	// eliminate it would keep re-enqueuing the same rule indefinitely.
	for _, declaration := range order {
		if candidate[declaration] && edges[declaration][declaration] {
			forbidden[declaration] = true
		}
	}
	position := make(map[ast.ASTHandle]int, len(order))
	for index, declaration := range order {
		position[declaration] = index
	}
	for {
		changed := false
		for _, component := range inlinerComponents(order, candidate, forbidden, edges) {
			if len(component) <= 1 {
				continue
			}
			selected := component[0]
			for _, declaration := range component[1:] {
				if position[declaration] < position[selected] {
					selected = declaration
				}
			}
			forbidden[selected] = true
			changed = true
		}
		if !changed {
			return forbidden
		}
	}
}

func inlinerTopologicalOrder(order []ast.ASTHandle, candidate, forbidden map[ast.ASTHandle]bool, edges map[ast.ASTHandle]map[ast.ASTHandle]bool) []ast.ASTHandle {
	visited := make(map[ast.ASTHandle]bool)
	var result []ast.ASTHandle
	var visit func(ast.ASTHandle)
	visit = func(declaration ast.ASTHandle) {
		if visited[declaration] || !candidate[declaration] || forbidden[declaration] {
			return
		}
		visited[declaration] = true
		for _, dependency := range order {
			if edges[declaration][dependency] {
				visit(dependency)
			}
		}
		result = append(result, declaration)
	}
	for _, declaration := range order {
		visit(declaration)
	}
	return result
}

func (p *RuleInlinerPlugin) forbidMultipleMultipliers(source *base.RuleSet, order []ast.ASTHandle, candidate, forbidden map[ast.ASTHandle]bool, edges map[ast.ASTHandle]map[ast.ASTHandle]bool, headCount, tailCount map[ast.ASTHandle]int, definitions map[ast.ASTHandle][]*base.Rule) error {
	allowed := func(declaration ast.ASTHandle) bool {
		return candidate[declaration] && !forbidden[declaration]
	}
	for _, head := range inlinerTopologicalOrder(order, candidate, forbidden, edges) {
		multiHead := headCount[head] > 1
		multiOccurrence := tailCount[head] > 1
		nextHead := false
		for _, rule := range definitions[head] {
			positive, err := rule.PositiveTailSize()
			if err != nil {
				return err
			}
			for tailIndex := 0; tailIndex < positive; tailIndex++ {
				tail, err := rule.TailDeclaration(tailIndex)
				if err != nil {
					return err
				}
				if !allowed(tail) || headCount[tail] <= 1 {
					continue
				}
				if multiHead {
					forbidden[head] = true
					nextHead = true
					break
				}
				if multiOccurrence {
					forbidden[tail] = true
					continue
				}
				multiHead = true
				headCount[head] *= headCount[tail]
			}
			if nextHead {
				break
			}
		}
	}

	count, err := source.Len()
	if err != nil {
		return err
	}
	for index := 0; index < count; index++ {
		rule, getErr := source.Rule(index)
		if getErr != nil {
			return getErr
		}
		head, getErr := rule.HeadDeclaration()
		positive, positiveErr := rule.PositiveTailSize()
		if getErr != nil || positiveErr != nil {
			return errors.Join(getErr, positiveErr, rule.Close())
		}
		if allowed(head) {
			if getErr = rule.Close(); getErr != nil {
				return getErr
			}
			continue
		}
		hasMultiplier := false
		for tailIndex := 0; tailIndex < positive; tailIndex++ {
			tail, tailErr := rule.TailDeclaration(tailIndex)
			if tailErr != nil {
				getErr = tailErr
				break
			}
			if !allowed(tail) || headCount[tail] <= 1 {
				continue
			}
			if hasMultiplier {
				forbidden[tail] = true
			} else {
				hasMultiplier = true
			}
		}
		if getErr = errors.Join(getErr, rule.Close()); getErr != nil {
			return getErr
		}
	}
	return nil
}

func (p *RuleInlinerPlugin) expandDefinition(rule *base.Rule, allowed, processed map[ast.ASTHandle]bool, definitions map[ast.ASTHandle][]*base.Rule) (_ []*base.Rule, resultErr error) {
	initial, err := rule.Retain()
	if err != nil {
		return nil, err
	}
	queue := []*base.Rule{initial}
	var result []*base.Rule
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, closeRules(queue), closeRules(result))
		}
	}()
	for len(queue) != 0 {
		current := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		positive, currentErr := current.PositiveTailSize()
		inlineIndex := -1
		var declaration ast.ASTHandle
		for tailIndex := 0; currentErr == nil && tailIndex < positive; tailIndex++ {
			declaration, currentErr = current.TailDeclaration(tailIndex)
			if currentErr == nil && allowed[declaration] && processed[declaration] {
				inlineIndex = tailIndex
				break
			}
		}
		if currentErr != nil {
			return nil, errors.Join(currentErr, current.Close())
		}
		if inlineIndex < 0 {
			result = append(result, current)
			continue
		}
		for _, definition := range definitions[declaration] {
			resolved, resolveErr := p.resolve(current, inlineIndex, definition)
			if resolveErr != nil {
				return nil, errors.Join(resolveErr, current.Close())
			}
			if resolved != nil {
				queue = append(queue, resolved)
			}
		}
		if currentErr = current.Close(); currentErr != nil {
			return nil, currentErr
		}
	}
	return result, nil
}

func (p *RuleInlinerPlugin) accumulateDefinitions(order []ast.ASTHandle, allowed map[ast.ASTHandle]bool, edges map[ast.ASTHandle]map[ast.ASTHandle]bool, original map[ast.ASTHandle][]*base.Rule) (_ map[ast.ASTHandle][]*base.Rule, resultErr error) {
	result := make(map[ast.ASTHandle][]*base.Rule)
	processed := make(map[ast.ASTHandle]bool)
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, closeRuleBuckets(result))
		}
	}()
	for _, declaration := range inlinerTopologicalOrder(order, allowed, nil, edges) {
		for _, rule := range original[declaration] {
			expanded, err := p.expandDefinition(rule, allowed, processed, result)
			if err != nil {
				return nil, err
			}
			result[declaration] = append(result[declaration], expanded...)
		}
		processed[declaration] = true
	}
	return result, nil
}

func (p *RuleInlinerPlugin) plan(source *base.RuleSet) (map[ast.ASTHandle]bool, map[ast.ASTHandle][]*base.Rule, error) {
	count, err := source.Len()
	if err != nil {
		return nil, nil, err
	}
	headCount := make(map[ast.ASTHandle]int)
	tailCount := make(map[ast.ASTHandle]int)
	negative := make(map[ast.ASTHandle]bool)
	edges := make(map[ast.ASTHandle]map[ast.ASTHandle]bool)
	definitions := make(map[ast.ASTHandle][]*base.Rule)
	var declarationOrder []ast.ASTHandle
	for index := 0; index < count; index++ {
		rule, getErr := source.Rule(index)
		if getErr != nil {
			return nil, nil, errors.Join(getErr, closeRuleBuckets(definitions))
		}
		head, getErr := rule.HeadDeclaration()
		if getErr != nil {
			return nil, nil, errors.Join(getErr, rule.Close(), closeRuleBuckets(definitions))
		}
		if headCount[head] == 0 {
			declarationOrder = append(declarationOrder, head)
		}
		headCount[head]++
		definitions[head] = append(definitions[head], rule)
		uninterpreted, getErr := rule.UninterpretedTailSize()
		positive, positiveErr := rule.PositiveTailSize()
		if getErr != nil || positiveErr != nil {
			return nil, nil, errors.Join(getErr, positiveErr, closeRuleBuckets(definitions))
		}
		for tailIndex := 0; tailIndex < uninterpreted; tailIndex++ {
			declaration, tailErr := rule.TailDeclaration(tailIndex)
			if tailErr != nil {
				return nil, nil, errors.Join(tailErr, closeRuleBuckets(definitions))
			}
			tailCount[declaration]++
			if tailIndex >= positive {
				negative[declaration] = true
			}
			if edges[head] == nil {
				edges[head] = make(map[ast.ASTHandle]bool)
			}
			edges[head][declaration] = true
		}
	}
	candidate := make(map[ast.ASTHandle]bool)
	for declaration, heads := range headCount {
		candidate[declaration] = !p.protected[declaration] && !negative[declaration] && (heads <= 1 || tailCount[declaration] <= 1 && heads <= 4)
	}
	forbidden := breakInliningCycles(declarationOrder, candidate, edges)
	if err = p.forbidMultipleMultipliers(source, declarationOrder, candidate, forbidden, edges, headCount, tailCount, definitions); err != nil {
		return nil, nil, errors.Join(err, closeRuleBuckets(definitions))
	}
	allowed := make(map[ast.ASTHandle]bool)
	for declaration := range candidate {
		allowed[declaration] = candidate[declaration] && !forbidden[declaration]
	}
	expanded, err := p.accumulateDefinitions(declarationOrder, allowed, edges, definitions)
	if err != nil {
		return nil, nil, errors.Join(err, closeRuleBuckets(definitions))
	}
	if err = closeRuleBuckets(definitions); err != nil {
		return nil, nil, errors.Join(err, closeRuleBuckets(expanded))
	}
	return allowed, expanded, nil
}

func closeRuleBuckets(buckets map[ast.ASTHandle][]*base.Rule) (result error) {
	for _, rules := range buckets {
		result = errors.Join(result, closeRules(rules))
	}
	return result
}

func (p *RuleInlinerPlugin) rulesUnify(target *base.Rule, tailIndex int, source *base.Rule) (_ bool, resultErr error) {
	tail, negated, err := target.Tail(tailIndex)
	if err != nil || negated {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return false, err
	}
	sourceHead, err := source.Head()
	if err != nil {
		return false, err
	}
	targetVariables, err := ruleVariableCount(p.manager, target)
	if err != nil {
		return false, err
	}
	sourceVariables, err := ruleVariableCount(p.manager, source)
	if err != nil {
		return false, err
	}
	variables := targetVariables
	if sourceVariables > variables {
		variables = sourceVariables
	}
	subst, err := substitution.New(p.manager)
	if err != nil {
		return false, err
	}
	unifier, err := substitution.NewUnifier(p.manager)
	if err != nil {
		return false, errors.Join(err, subst.Close())
	}
	defer func() { resultErr = errors.Join(resultErr, unifier.Close(), subst.Close()) }()
	if err = subst.Reserve(2, variables); err != nil {
		return false, err
	}
	return unifier.UnifyPair(tail, sourceHead, subst, true)
}

func (p *RuleInlinerPlugin) orientedDefinition(rule *base.Rule, rules *base.RuleSet) (bool, error) {
	head, err := rule.HeadDeclaration()
	if err != nil {
		return false, err
	}
	headStratum, err := rules.PredicateStratum(head)
	if err != nil {
		return false, err
	}
	headArity, err := p.manager.GetArity(head)
	if err != nil {
		return false, err
	}
	headID, err := p.manager.GetASTID(head)
	if err != nil {
		return false, err
	}
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return false, err
	}
	for index := 0; index < positive; index++ {
		tail, getErr := rule.TailDeclaration(index)
		if getErr != nil {
			return false, getErr
		}
		tailStratum, getErr := rules.PredicateStratum(tail)
		if getErr != nil {
			return false, getErr
		}
		if tailStratum != headStratum {
			continue
		}
		tailArity, getErr := p.manager.GetArity(tail)
		if getErr != nil {
			return false, getErr
		}
		if tailArity > headArity {
			return false, nil
		}
		if tailArity == headArity {
			tailID, idErr := p.manager.GetASTID(tail)
			if idErr != nil {
				return false, idErr
			}
			if tailID >= headID {
				return false, nil
			}
		}
	}
	return true, nil
}

func (p *RuleInlinerPlugin) orientedEagerRule(rule *base.Rule, rules *base.RuleSet) (_ *base.Rule, changed bool, resultErr error) {
	negated, err := rule.HasNegation()
	if err != nil || negated {
		return nil, false, err
	}
	head, err := rule.HeadDeclaration()
	if err != nil {
		return nil, false, err
	}
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return nil, false, err
	}
	for tailIndex := 0; tailIndex < positive; tailIndex++ {
		declaration, getErr := rule.TailDeclaration(tailIndex)
		if getErr != nil {
			return nil, false, getErr
		}
		if declaration == head {
			continue
		}
		definitions, getErr := rules.PredicateRules(declaration)
		if getErr != nil {
			return nil, false, getErr
		}
		candidate := -1
		if len(definitions) == 1 {
			candidate = 0
		} else {
			for index, definition := range definitions {
				unifies, unifyErr := p.rulesUnify(rule, tailIndex, definition)
				if unifyErr != nil {
					return nil, false, errors.Join(unifyErr, closeRules(definitions))
				}
				if !unifies {
					continue
				}
				if candidate >= 0 {
					candidate = -2
					break
				}
				candidate = index
			}
		}
		if candidate == -2 {
			if getErr = closeRules(definitions); getErr != nil {
				return nil, false, getErr
			}
			continue
		}
		if candidate < 0 {
			return nil, true, closeRules(definitions)
		}
		oriented, orientationErr := p.orientedDefinition(definitions[candidate], rules)
		if orientationErr != nil || !oriented {
			getErr = errors.Join(orientationErr, closeRules(definitions))
			if getErr != nil {
				return nil, false, getErr
			}
			continue
		}
		resolved, resolveErr := p.resolve(rule, tailIndex, definitions[candidate])
		resolveErr = errors.Join(resolveErr, closeRules(definitions))
		if resolveErr != nil {
			return nil, false, resolveErr
		}
		return resolved, true, nil
	}
	return nil, false, nil
}

func (p *RuleInlinerPlugin) orientedEager(source *base.RuleSet) (_ *base.RuleSet, changed bool, resultErr error) {
	count, err := source.Len()
	if err != nil {
		return nil, false, err
	}
	result, err := base.NewRuleSet(p.rules)
	if err != nil {
		return nil, false, err
	}
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	if err = result.InheritPredicates(source); err != nil {
		return nil, false, err
	}
	for index := 0; index < count; index++ {
		current, getErr := source.Rule(index)
		if getErr != nil {
			return nil, false, getErr
		}
		for {
			replacement, step, stepErr := p.orientedEagerRule(current, source)
			if stepErr != nil {
				return nil, false, errors.Join(stepErr, current.Close())
			}
			if !step {
				break
			}
			changed = true
			if getErr = current.Close(); getErr != nil {
				return nil, false, errors.Join(getErr, closeRuleIf(replacement))
			}
			current = replacement
			if current == nil {
				break
			}
		}
		if current != nil {
			getErr = errors.Join(result.Add(current), current.Close())
			if getErr != nil {
				return nil, false, getErr
			}
		}
	}
	keep = true
	return result, changed, nil
}

func (p *RuleInlinerPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	count, err := source.Len()
	if err != nil || count == 0 {
		return nil, err
	}
	// The source refuses the complete inliner when any interpreted tail has a
	// quantifier; mixing transformed and untransformed rules is not supported.
	for index := 0; index < count; index++ {
		rule, getErr := source.Rule(index)
		if getErr != nil {
			return nil, getErr
		}
		uninterpreted, getErr := rule.UninterpretedTailSize()
		total, totalErr := rule.TailSize()
		quantified := false
		for tailIndex := uninterpreted; getErr == nil && totalErr == nil && tailIndex < total; tailIndex++ {
			tail, _, tailErr := rule.Tail(tailIndex)
			if tailErr != nil {
				getErr = tailErr
				break
			}
			quantified, tailErr = p.manager.HasQuantifiers(tail)
			if tailErr != nil {
				getErr = tailErr
				break
			}
			if quantified {
				break
			}
		}
		getErr = errors.Join(getErr, totalErr, rule.Close())
		if getErr != nil {
			return nil, getErr
		}
		if quantified {
			return nil, nil
		}
	}
	if !p.eager {
		if !p.linear {
			return nil, nil
		}
		return p.inlineLinear(source)
	}
	allowed, definitions, err := p.plan(source)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, closeRuleBuckets(definitions)) }()
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
	changed := false
	for index := 0; index < count; index++ {
		rule, getErr := source.Rule(index)
		if getErr != nil {
			return nil, getErr
		}
		head, getErr := rule.HeadDeclaration()
		if getErr != nil {
			return nil, errors.Join(getErr, rule.Close())
		}
		if allowed[head] {
			changed = true
			if getErr = rule.Close(); getErr != nil {
				return nil, getErr
			}
			continue
		}
		queue := []*base.Rule{rule}
		for len(queue) != 0 {
			current := queue[len(queue)-1]
			queue = queue[:len(queue)-1]
			positive, currentErr := current.PositiveTailSize()
			inlineIndex := -1
			var declaration ast.ASTHandle
			for tailIndex := 0; currentErr == nil && tailIndex < positive; tailIndex++ {
				declaration, currentErr = current.TailDeclaration(tailIndex)
				if currentErr == nil && allowed[declaration] {
					inlineIndex = tailIndex
					break
				}
			}
			if currentErr != nil {
				return nil, errors.Join(currentErr, current.Close(), closeRules(queue))
			}
			if inlineIndex < 0 {
				currentErr = result.Add(current)
				currentErr = errors.Join(currentErr, current.Close())
				if currentErr != nil {
					return nil, errors.Join(currentErr, closeRules(queue))
				}
				continue
			}
			changed = true
			for _, definition := range definitions[declaration] {
				resolved, resolveErr := p.resolve(current, inlineIndex, definition)
				if resolveErr != nil {
					currentName, _ := current.Name()
					definitionName, _ := definition.Name()
					return nil, errors.Join(fmt.Errorf("muz/transforms: inline %s with %s: %w", currentName, definitionName, resolveErr), current.Close(), closeRules(queue))
				}
				if resolved != nil {
					queue = append(queue, resolved)
				}
			}
			if currentErr = current.Close(); currentErr != nil {
				return nil, errors.Join(currentErr, closeRules(queue))
			}
		}
	}
	valid, err := result.Finalize()
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, fmt.Errorf("muz/transforms: eager inliner produced unstratified rules")
	}
	oriented, orientedChanged, err := p.orientedEager(result)
	if err != nil {
		return nil, err
	}
	if orientedChanged {
		previous := result
		result = oriented
		if err = previous.Close(); err != nil {
			return nil, err
		}
		changed = true
	} else {
		if err = oriented.Close(); err != nil {
			return nil, err
		}
		if err = result.Reopen(); err != nil {
			return nil, err
		}
	}
	if !changed {
		if !p.linear {
			return nil, nil
		}
		linear, linearErr := p.inlineLinear(source)
		if linearErr != nil {
			return nil, linearErr
		}
		if linear == nil {
			return nil, nil
		}
		if closeErr := result.Close(); closeErr != nil {
			return nil, errors.Join(closeErr, linear.Close())
		}
		keep = true
		return linear, nil
	}
	if p.linear {
		linear, linearErr := p.inlineLinear(result)
		if linearErr != nil {
			return nil, linearErr
		}
		if linear != nil {
			if err = result.Close(); err != nil {
				return nil, errors.Join(err, linear.Close())
			}
			result = linear
		}
	}
	keep = true
	return result, nil
}

// inlineLinear is the source straight-line relation pass. A defining rule is
// consumed only when its head unifies with one live tail position and that
// tail has one live defining rule, so every successful step removes a rule
// and cannot create an inlining cycle.
func (p *RuleInlinerPlugin) inlineLinear(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	count, err := source.Len()
	if err != nil || count == 0 {
		return nil, err
	}
	rules := make([]*base.Rule, count)
	valid := make([]bool, count)
	canRemove := make([]bool, count)
	canExpand := make([]bool, count)
	defer func() { resultErr = errors.Join(resultErr, closeRules(rules)) }()
	for index := 0; index < count; index++ {
		rule, getErr := source.Rule(index)
		if getErr != nil {
			return nil, getErr
		}
		rules[index], valid[index] = rule, true
		head, getErr := rule.HeadDeclaration()
		if getErr != nil {
			return nil, getErr
		}
		canRemove[index] = !p.protected[head]
		uninterpreted, getErr := rule.UninterpretedTailSize()
		positive, positiveErr := rule.PositiveTailSize()
		if getErr != nil || positiveErr != nil {
			return nil, errors.Join(getErr, positiveErr)
		}
		if uninterpreted == 1 && positive == 1 {
			tail, declarationErr := rule.TailDeclaration(0)
			if declarationErr != nil {
				return nil, declarationErr
			}
			canExpand[index] = !p.protected[tail]
		}
	}
	changed := false
	for index := 0; index < count; index++ {
		for valid[index] && canExpand[index] {
			current := rules[index]
			declaration, getErr := current.TailDeclaration(0)
			if getErr != nil {
				return nil, getErr
			}
			candidateIndex := -1
			var replacement *base.Rule
			for definitionIndex, definition := range rules {
				if !valid[definitionIndex] {
					continue
				}
				head, headErr := definition.HeadDeclaration()
				if headErr != nil {
					return nil, errors.Join(headErr, closeRuleIf(replacement))
				}
				if head != declaration {
					continue
				}
				resolved, resolveErr := p.resolve(current, 0, definition)
				if resolveErr != nil {
					return nil, errors.Join(resolveErr, closeRuleIf(replacement))
				}
				if resolved == nil {
					continue
				}
				if candidateIndex >= 0 {
					if closeErr := errors.Join(resolved.Close(), replacement.Close()); closeErr != nil {
						return nil, closeErr
					}
					replacement = nil
					candidateIndex = -2
					break
				}
				candidateIndex, replacement = definitionIndex, resolved
			}
			if candidateIndex < 0 {
				if replacement != nil {
					if getErr = replacement.Close(); getErr != nil {
						return nil, getErr
					}
				}
				break
			}
			if candidateIndex == index || !canRemove[candidateIndex] {
				if getErr = replacement.Close(); getErr != nil {
					return nil, getErr
				}
				break
			}
			matches := 0
			for targetIndex, target := range rules {
				if !valid[targetIndex] {
					continue
				}
				positive, positiveErr := target.PositiveTailSize()
				if positiveErr != nil {
					return nil, errors.Join(positiveErr, replacement.Close())
				}
				for tailIndex := 0; tailIndex < positive; tailIndex++ {
					tailDeclaration, tailErr := target.TailDeclaration(tailIndex)
					if tailErr != nil {
						return nil, errors.Join(tailErr, replacement.Close())
					}
					if tailDeclaration != declaration {
						continue
					}
					probe, probeErr := p.resolve(target, tailIndex, rules[candidateIndex])
					if probeErr != nil {
						return nil, errors.Join(probeErr, replacement.Close())
					}
					if probe != nil {
						matches++
						if probeErr = probe.Close(); probeErr != nil {
							return nil, errors.Join(probeErr, replacement.Close())
						}
					}
				}
			}
			if matches != 1 {
				if getErr = replacement.Close(); getErr != nil {
					return nil, getErr
				}
				break
			}
			if getErr = current.Close(); getErr != nil {
				return nil, errors.Join(getErr, replacement.Close())
			}
			rules[index] = replacement
			valid[candidateIndex] = false
			canExpand[index] = canExpand[candidateIndex]
			changed = true
		}
	}
	if !changed {
		return nil, nil
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
	for index, rule := range rules {
		if valid[index] {
			if err = result.Add(rule); err != nil {
				return nil, err
			}
		}
	}
	keep = true
	return result, nil
}

func closeRuleIf(rule *base.Rule) error {
	if rule == nil {
		return nil
	}
	return rule.Close()
}

func (p *RuleInlinerPlugin) Close() error {
	if p == nil {
		return base.ErrRuleTransformerOwner
	}
	p.closed = true
	p.protected = nil
	p.params = nil
	p.globals = nil
	if p.simplifier != nil {
		err := p.simplifier.Close()
		p.simplifier = nil
		return err
	}
	return nil
}
