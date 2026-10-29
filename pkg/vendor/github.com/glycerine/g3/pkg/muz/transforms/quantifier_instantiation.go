package transforms

// Source: src/muz/transforms/dl_mk_quantifier_instantiation.cpp/.h retained
// Horn-clause E-matching instantiation.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/pattern"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/backtrack"
)

type QuantifierInstantiationPlugin struct {
	manager  *ast.ASTManager
	rules    *base.RuleManager
	priority uint32
	closed   bool
}

func NewQuantifierInstantiationPlugin(manager *ast.ASTManager, rules *base.RuleManager, priority uint32) *QuantifierInstantiationPlugin {
	return &QuantifierInstantiationPlugin{manager: manager, rules: rules, priority: priority}
}

func (p *QuantifierInstantiationPlugin) Priority() uint32            { return p.priority }
func (p *QuantifierInstantiationPlugin) CanDestratifyNegation() bool { return false }
func (p *QuantifierInstantiationPlugin) Cancel()                     {}

type quantifierTermPair struct{ pattern, term ast.ASTHandle }

type quantifierEGraph struct {
	manager   *ast.ASTManager
	classes   *backtrack.BasicUnionFind
	terms     map[uint32]ast.ASTHandle
	functions map[ast.ASTHandle][]ast.ASTHandle
}

func newQuantifierEGraph(manager *ast.ASTManager) *quantifierEGraph {
	return &quantifierEGraph{
		manager: manager, classes: backtrack.NewBasicUnionFind(),
		terms: make(map[uint32]ast.ASTHandle), functions: make(map[ast.ASTHandle][]ast.ASTHandle),
	}
}

func (g *quantifierEGraph) binaryEquality(expression ast.ASTHandle) (left, right ast.ASTHandle, yes bool, err error) {
	kind, err := g.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return left, right, false, err
	}
	family, err := g.manager.GetFamilyID(expression)
	if err != nil || family != g.manager.BasicFamilyID() {
		return left, right, false, err
	}
	op, err := g.manager.GetDeclKind(expression)
	if err != nil || op != ast.ASTOpEq && op != ast.ASTOpIff {
		return left, right, false, err
	}
	count, err := g.manager.GetNumArgs(expression)
	if err != nil || count != 2 {
		return left, right, false, err
	}
	left, err = g.manager.GetArg(expression, 0)
	if err == nil {
		right, err = g.manager.GetArg(expression, 1)
	}
	return left, right, err == nil, err
}

func (g *quantifierEGraph) collect(root ast.ASTHandle) error {
	visited := make(map[ast.ASTHandle]bool)
	todo := []ast.ASTHandle{root}
	for len(todo) != 0 {
		last := len(todo) - 1
		expression := todo[last]
		todo = todo[:last]
		if visited[expression] {
			continue
		}
		visited[expression] = true
		id, err := g.manager.GetASTID(expression)
		if err != nil {
			return err
		}
		g.terms[id] = expression
		left, right, equality, err := g.binaryEquality(expression)
		if err != nil {
			return err
		}
		if equality {
			leftID, getErr := g.manager.GetASTID(left)
			if getErr != nil {
				return getErr
			}
			rightID, getErr := g.manager.GetASTID(right)
			if getErr != nil {
				return getErr
			}
			g.classes.Merge(leftID, rightID)
		}
		kind, err := g.manager.GetKind(expression)
		if err != nil {
			return err
		}
		if kind != ast.ASTApp {
			continue
		}
		declaration, err := g.manager.GetDecl(expression)
		if err != nil {
			return err
		}
		g.functions[declaration] = append(g.functions[declaration], expression)
		count, err := g.manager.GetNumArgs(expression)
		if err != nil {
			return err
		}
		for i := uint32(0); i < count; i++ {
			argument, getErr := g.manager.GetArg(expression, i)
			if getErr != nil {
				return getErr
			}
			todo = append(todo, argument)
		}
	}
	return nil
}

func (g *quantifierEGraph) equivalents(term ast.ASTHandle) ([]ast.ASTHandle, error) {
	id, err := g.manager.GetASTID(term)
	if err != nil {
		return nil, err
	}
	result := []ast.ASTHandle{term}
	next := g.classes.Next(id)
	for next != id {
		if equivalent := g.terms[next]; !equivalent.IsNull() {
			result = append(result, equivalent)
		}
		next = g.classes.Next(next)
	}
	return result, nil
}

func (p *QuantifierInstantiationPlugin) forallConjunct(expression ast.ASTHandle) (ast.ASTHandle, bool, error) {
	kind, err := p.manager.GetKind(expression)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if kind == ast.ASTApp {
		family, getErr := p.manager.GetFamilyID(expression)
		if getErr != nil {
			return ast.ASTHandle{}, false, getErr
		}
		op, getErr := p.manager.GetDeclKind(expression)
		if getErr != nil {
			return ast.ASTHandle{}, false, getErr
		}
		if family == p.manager.BasicFamilyID() && op == ast.ASTOpIff {
			left, getErr := p.manager.GetArg(expression, 0)
			if getErr != nil {
				return ast.ASTHandle{}, false, getErr
			}
			right, getErr := p.manager.GetArg(expression, 1)
			if getErr != nil {
				return ast.ASTHandle{}, false, getErr
			}
			if p.manager.IsTrue(right) {
				expression = left
			} else if p.manager.IsTrue(left) {
				expression = right
			}
			kind, err = p.manager.GetKind(expression)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
		}
	}
	if kind != ast.ASTQuantifier {
		return ast.ASTHandle{}, false, nil
	}
	forall, err := p.manager.IsForall(expression)
	return expression, forall, err
}

func (p *QuantifierInstantiationPlugin) ruleVariables(rule *base.Rule) ([]ast.ASTHandle, error) {
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
	total, err := rule.TailSize()
	if err != nil {
		return nil, err
	}
	for i := 0; i < total; i++ {
		tail, _, getErr := rule.Tail(i)
		if getErr != nil {
			return nil, getErr
		}
		if getErr = used.Process(tail); getErr != nil {
			return nil, getErr
		}
	}
	variables := make([]ast.ASTHandle, used.MaxFoundVarIndexPlusOne())
	for i := range variables {
		sort := used.Contains(uint32(i))
		if sort.IsNull() {
			continue
		}
		variables[i], err = p.manager.MkVar(uint32(i), sort)
		if err != nil {
			return nil, err
		}
	}
	return variables, nil
}

func (p *QuantifierInstantiationPlugin) patternedQuantifier(q ast.ASTHandle) (_ ast.ASTRef, resultErr error) {
	count, err := p.manager.GetNumPatterns(q)
	if err != nil {
		return ast.ASTRef{}, err
	}
	result, err := p.manager.NewRef(q)
	if err != nil || count != 0 {
		return result, err
	}
	inference, err := pattern.NewInferenceRewriter(p.manager)
	if err != nil {
		return ast.ASTRef{}, errors.Join(err, p.manager.DelRef(&result))
	}
	defer func() { resultErr = errors.Join(resultErr, inference.Close()) }()
	if err = inference.Rewrite(q, result); err != nil {
		return ast.ASTRef{}, errors.Join(err, p.manager.DelRef(&result))
	}
	return result, nil
}

func (p *QuantifierInstantiationPlugin) instantiate(q ast.ASTHandle, graph *quantifierEGraph, constantsToVariables *astrewriter.ExprSafeReplace) (instances []ast.ASTRef, resultErr error) {
	patterned, err := p.patternedQuantifier(q)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&patterned)) }()
	q, err = p.manager.RefGet(patterned)
	if err != nil {
		return nil, err
	}
	declarations, err := p.manager.GetNumDeclarations(q)
	if err != nil {
		return nil, err
	}
	bindings := make([]ast.ASTHandle, declarations)
	yield := func() error {
		values := make([]ast.ASTHandle, len(bindings))
		for i := range bindings {
			if bindings[i].IsNull() {
				return ast.ErrASTDomain
			}
			values[len(bindings)-1-i] = bindings[i]
		}
		instance, makeErr := p.manager.NewRef(ast.ASTHandle{})
		if makeErr != nil {
			return makeErr
		}
		if makeErr = astrewriter.InstantiateQuantifier(p.manager, q, values, instance); makeErr == nil {
			makeErr = constantsToVariables.ApplyRef(instance)
		}
		if makeErr != nil {
			return errors.Join(makeErr, p.manager.DelRef(&instance))
		}
		instances = append(instances, instance)
		return nil
	}
	var matchPairs func([]quantifierTermPair, int, func() error) error
	matchPairs = func(pairs []quantifierTermPair, index int, done func() error) error {
		if index == len(pairs) {
			return done()
		}
		pair := pairs[index]
		kind, getErr := p.manager.GetKind(pair.pattern)
		if getErr != nil {
			return getErr
		}
		if kind == ast.ASTVar {
			variable, getErr := p.manager.GetVarIndex(pair.pattern)
			if getErr != nil || variable >= uint32(len(bindings)) {
				return errors.Join(getErr, ast.ErrASTDomain)
			}
			if !bindings[variable].IsNull() {
				return matchPairs(pairs, index+1, done)
			}
			bindings[variable] = pair.term
			getErr = matchPairs(pairs, index+1, done)
			bindings[variable] = ast.ASTHandle{}
			return getErr
		}
		if kind != ast.ASTApp {
			return nil
		}
		patternDecl, getErr := p.manager.GetDecl(pair.pattern)
		if getErr != nil {
			return getErr
		}
		patternArity, getErr := p.manager.GetNumArgs(pair.pattern)
		if getErr != nil {
			return getErr
		}
		equivalents, getErr := graph.equivalents(pair.term)
		if getErr != nil {
			return getErr
		}
		for _, term := range equivalents {
			termKind, branchErr := p.manager.GetKind(term)
			if branchErr != nil {
				return branchErr
			}
			if termKind != ast.ASTApp {
				continue
			}
			termDecl, branchErr := p.manager.GetDecl(term)
			if branchErr != nil || termDecl != patternDecl {
				if branchErr != nil {
					return branchErr
				}
				continue
			}
			termArity, branchErr := p.manager.GetNumArgs(term)
			if branchErr != nil {
				return branchErr
			}
			if termArity != patternArity {
				continue
			}
			expanded := append([]quantifierTermPair(nil), pairs...)
			for i := uint32(0); i < patternArity; i++ {
				patternArg, getErr := p.manager.GetArg(pair.pattern, i)
				if getErr != nil {
					return getErr
				}
				termArg, getErr := p.manager.GetArg(term, i)
				if getErr != nil {
					return getErr
				}
				expanded = append(expanded, quantifierTermPair{pattern: patternArg, term: termArg})
			}
			if branchErr = matchPairs(expanded, index+1, done); branchErr != nil {
				return branchErr
			}
		}
		return nil
	}
	patterns, err := p.manager.GetNumPatterns(q)
	if err != nil {
		return nil, err
	}
	for patternIndex := uint32(0); patternIndex < patterns; patternIndex++ {
		annotation, getErr := p.manager.GetPattern(q, patternIndex)
		if getErr != nil {
			return nil, getErr
		}
		triggers, getErr := p.manager.GetNumArgs(annotation)
		if getErr != nil {
			return nil, getErr
		}
		var matchTrigger func(uint32) error
		matchTrigger = func(index uint32) error {
			if index == triggers {
				return yield()
			}
			trigger, branchErr := p.manager.GetArg(annotation, index)
			if branchErr != nil {
				return branchErr
			}
			kind, branchErr := p.manager.GetKind(trigger)
			if branchErr != nil || kind != ast.ASTApp {
				return errors.Join(branchErr, ast.ErrASTDomain)
			}
			declaration, branchErr := p.manager.GetDecl(trigger)
			if branchErr != nil {
				return branchErr
			}
			for _, term := range graph.functions[declaration] {
				if branchErr = matchPairs([]quantifierTermPair{{pattern: trigger, term: term}}, 0, func() error { return matchTrigger(index + 1) }); branchErr != nil {
					return branchErr
				}
			}
			return nil
		}
		if err = matchTrigger(0); err != nil {
			return nil, err
		}
	}
	return instances, nil
}

func (p *QuantifierInstantiationPlugin) transformRule(rule *base.Rule) (_ *base.Rule, changed bool, resultErr error) {
	total, err := rule.TailSize()
	if err != nil {
		return nil, false, err
	}
	conjuncts := make([]ast.ASTHandle, 0, total)
	for i := 0; i < total; i++ {
		tail, _, getErr := rule.Tail(i)
		if getErr != nil {
			return nil, false, getErr
		}
		conjuncts = append(conjuncts, tail)
	}
	conjuncts, err = p.manager.FlattenAnd(conjuncts)
	if err != nil {
		return nil, false, err
	}
	quantifiers := make([]ast.ASTHandle, 0)
	for i := 0; i < len(conjuncts); i++ {
		q, forall, getErr := p.forallConjunct(conjuncts[i])
		if getErr != nil {
			return nil, false, getErr
		}
		if !forall {
			continue
		}
		quantifiers = append(quantifiers, q)
		conjuncts[i] = conjuncts[len(conjuncts)-1]
		conjuncts = conjuncts[:len(conjuncts)-1]
		i--
	}
	if len(quantifiers) == 0 {
		return nil, false, nil
	}
	variables, err := p.ruleVariables(rule)
	if err != nil {
		return nil, false, err
	}
	variablesToConstants, err := astrewriter.NewExprSafeReplace(p.manager)
	if err != nil {
		return nil, false, err
	}
	constantsToVariables, err := astrewriter.NewExprSafeReplace(p.manager)
	if err != nil {
		return nil, false, errors.Join(err, variablesToConstants.Close())
	}
	defer func() { resultErr = errors.Join(resultErr, constantsToVariables.Close(), variablesToConstants.Close()) }()
	for _, variable := range variables {
		if variable.IsNull() {
			continue
		}
		sort, getErr := p.manager.GetSort(variable)
		if getErr != nil {
			return nil, false, getErr
		}
		constant, getErr := p.manager.MkFreshConst("C", sort)
		if getErr != nil {
			return nil, false, getErr
		}
		if getErr = variablesToConstants.Insert(variable, constant); getErr == nil {
			getErr = constantsToVariables.Insert(constant, variable)
		}
		if getErr != nil {
			return nil, false, getErr
		}
	}
	body, err := p.manager.MkAndArgs(conjuncts)
	if err != nil {
		return nil, false, err
	}
	bodyRef, err := p.manager.NewRef(body)
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&bodyRef)) }()
	if err = variablesToConstants.ApplyRef(bodyRef); err != nil {
		return nil, false, err
	}
	body, err = p.manager.RefGet(bodyRef)
	if err != nil {
		return nil, false, err
	}
	graph := newQuantifierEGraph(p.manager)
	if err = graph.collect(body); err != nil {
		return nil, false, err
	}
	instanceRefs := make([]ast.ASTRef, 0)
	defer func() {
		for i := len(instanceRefs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&instanceRefs[i]))
		}
	}()
	for _, quantifier := range quantifiers {
		quantifierRef, makeErr := p.manager.NewRef(quantifier)
		if makeErr != nil {
			return nil, false, makeErr
		}
		if makeErr = variablesToConstants.ApplyRef(quantifierRef); makeErr != nil {
			return nil, false, errors.Join(makeErr, p.manager.DelRef(&quantifierRef))
		}
		quantifier, makeErr = p.manager.RefGet(quantifierRef)
		if makeErr != nil {
			return nil, false, errors.Join(makeErr, p.manager.DelRef(&quantifierRef))
		}
		instances, makeErr := p.instantiate(quantifier, graph, constantsToVariables)
		makeErr = errors.Join(makeErr, p.manager.DelRef(&quantifierRef))
		if makeErr != nil {
			for i := len(instances) - 1; i >= 0; i-- {
				makeErr = errors.Join(makeErr, p.manager.DelRef(&instances[i]))
			}
			return nil, false, makeErr
		}
		instanceRefs = append(instanceRefs, instances...)
	}
	for _, instance := range instanceRefs {
		handle, getErr := p.manager.RefGet(instance)
		if getErr != nil {
			return nil, false, getErr
		}
		conjuncts = append(conjuncts, handle)
	}
	head, err := rule.Head()
	if err != nil {
		return nil, false, err
	}
	name, err := rule.Name()
	if err != nil {
		return nil, false, err
	}
	replacement, err := p.rules.NewRule(head, conjuncts, nil, name)
	return replacement, true, err
}

func (p *QuantifierInstantiationPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	hasQuantifiers := false
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
		if getErr == nil {
			total, sizeErr := rule.TailSize()
			getErr = sizeErr
			for j := 0; getErr == nil && j < total; j++ {
				tail, _, tailErr := rule.Tail(j)
				getErr = tailErr
				if tailErr == nil {
					var quantified bool
					quantified, getErr = p.manager.HasQuantifiers(tail)
					hasQuantifiers = hasQuantifiers || quantified
				}
			}
		}
		getErr = errors.Join(getErr, rule.Close())
		if getErr != nil {
			return nil, getErr
		}
	}
	if !hasQuantifiers {
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
	changed := false
	for i := 0; i < count; i++ {
		rule, getErr := source.Rule(i)
		if getErr != nil {
			return nil, getErr
		}
		replacement, ruleChanged, transformErr := p.transformRule(rule)
		changed = changed || ruleChanged
		if transformErr == nil {
			if replacement == nil {
				replacement = rule
			}
			transformErr = result.Add(replacement)
		}
		transformErr = errors.Join(transformErr, rule.Close())
		if replacement != nil && replacement != rule {
			transformErr = errors.Join(transformErr, replacement.Close())
		}
		if transformErr != nil {
			return nil, transformErr
		}
	}
	if !changed {
		return nil, nil
	}
	keep = true
	return result, nil
}

func (p *QuantifierInstantiationPlugin) Close() error {
	if p == nil {
		return base.ErrRuleTransformerOwner
	}
	p.closed = true
	return nil
}
