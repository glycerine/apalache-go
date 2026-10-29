package spacer

// Source: src/muz/spacer/spacer_context.cpp pred_transformer::initialize,
// init_rules, init_rule, ground_free_vars, and init_atom.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/tactic"
	tacticcore "github.com/glycerine/g3/pkg/tactic/core"
)

type spacerNormalizedRule struct {
	rule         *base.Rule
	transition   ast.ASTHandle
	tag          ast.ASTHandle
	instances    []ast.ASTHandle
	auxiliaries  []ast.ASTHandle
	predecessors []*PredicateTransformer
	initial      bool
}

type spacerRuleBuilder struct {
	transformer *PredicateTransformer
	all         map[ast.ASTHandle]*PredicateTransformer
	theory      *rewriter.TheoryRewriter
	references  []ast.ASTRef
	rules       []*spacerNormalizedRule
}

func (b *spacerRuleBuilder) pin(handle ast.ASTHandle) error {
	reference, err := b.transformer.manager.NewRef(handle)
	if err == nil {
		b.references = append(b.references, reference)
	}
	return err
}

func (b *spacerRuleBuilder) fresh(prefix string, sort ast.ASTHandle) (ast.ASTHandle, error) {
	value, err := b.transformer.manager.MkFreshConst(prefix, sort)
	if err == nil {
		err = b.pin(value)
	}
	return value, err
}

func (b *spacerRuleBuilder) named(name string, sort ast.ASTHandle) (ast.ASTHandle, error) {
	symbol, err := b.transformer.manager.Symbols.Intern(name)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	value, err := b.transformer.manager.MkConst(symbol, sort)
	if err == nil {
		err = b.pin(value)
	}
	return value, err
}

func (b *spacerRuleBuilder) rewrite(formula ast.ASTHandle) (ast.ASTHandle, error) {
	out, err := b.transformer.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = b.theory.Rewrite(formula, out); err != nil {
		return ast.ASTHandle{}, errors.Join(err, b.transformer.manager.DelRef(&out))
	}
	result, err := b.transformer.manager.RefGet(out)
	if err != nil {
		return ast.ASTHandle{}, errors.Join(err, b.transformer.manager.DelRef(&out))
	}
	b.references = append(b.references, out)
	return result, nil
}

func (b *spacerRuleBuilder) blastTermITE(formula ast.ASTHandle) (_ ast.ASTHandle, resultErr error) {
	goal, err := tactic.NewGoal(b.transformer.manager, false, false, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, goal.Close()) }()
	if err = goal.Assert(formula); err != nil {
		return ast.ASTHandle{}, err
	}
	config := b.transformer.owner.contexts[0].config
	runner, err := tacticcore.NewEliminateTermITETactic(b.transformer.manager, config.Params, config.Globals, nil, config.ModelEvaluator)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = runner.IncRef(); err != nil {
		return ast.ASTHandle{}, errors.Join(err, runner.Close())
	}
	defer func() { resultErr = errors.Join(resultErr, runner.DecRef()) }()
	output := &tactic.TacticResult{}
	defer func() { resultErr = errors.Join(resultErr, output.Close()) }()
	if err = runner.Apply(goal, output); err != nil {
		return ast.ASTHandle{}, err
	}
	if len(output.Goals) != 1 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	formulas, err := output.Goals[0].AppendFormulas(nil)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := b.transformer.manager.MkAndArray(formulas)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = b.pin(result); err != nil {
		return ast.ASTHandle{}, err
	}
	return result, nil
}

func (b *spacerRuleBuilder) signature(transformer *PredicateTransformer) ([]ast.ASTHandle, error) {
	return transformer.Signature()
}

func (b *spacerRuleBuilder) stateApplication(transformer *PredicateTransformer, argument uint32, old *uint32) (ast.ASTHandle, error) {
	signature, err := b.signature(transformer)
	if err != nil || argument >= uint32(len(signature)) {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, err
	}
	declaration := signature[argument]
	if old == nil {
		declaration, err = b.transformer.owner.OldToNew(declaration, 0)
	} else {
		declaration, err = b.transformer.owner.OldToOld(declaration, 0, *old)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	application, err := b.transformer.manager.MkApp(declaration, nil)
	if err == nil {
		err = b.pin(application)
	}
	return application, err
}

func (b *spacerRuleBuilder) initializeAtom(atom ast.ASTHandle, variables *[]ast.ASTHandle, conjunction *[]ast.ASTHandle, old *uint32) (*PredicateTransformer, error) {
	declaration, err := b.transformer.manager.GetDecl(atom)
	if err != nil {
		return nil, err
	}
	owner := b.all[declaration]
	if owner == nil || owner.owner != b.transformer.owner {
		return nil, fmt.Errorf("%w: missing Spacer predicate transformer", ast.ErrASTDomain)
	}
	signature, err := owner.Signature()
	if err != nil {
		return nil, err
	}
	count, err := b.transformer.manager.GetNumArgs(atom)
	if err != nil || count != uint32(len(signature)) {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return nil, err
	}
	for index := uint32(0); index < count; index++ {
		representative, err := b.stateApplication(owner, index, old)
		if err != nil {
			return nil, err
		}
		argument, err := b.transformer.manager.GetArg(atom, index)
		if err != nil {
			return nil, err
		}
		kind, err := b.transformer.manager.GetKind(argument)
		if err != nil {
			return nil, err
		}
		if kind == ast.ASTVar {
			variable, err := b.transformer.manager.GetVarIndex(argument)
			if err != nil {
				return nil, err
			}
			for uint32(len(*variables)) <= variable {
				*variables = append(*variables, ast.ASTHandle{})
			}
			if prior := (*variables)[variable]; prior.IsNull() {
				(*variables)[variable] = representative
			} else {
				equality, err := b.transformer.manager.MkEq(representative, prior)
				if err != nil {
					return nil, err
				}
				if err = b.pin(equality); err != nil {
					return nil, err
				}
				*conjunction = append(*conjunction, equality)
			}
			continue
		}
		equality, err := b.transformer.manager.MkEq(representative, argument)
		if err != nil {
			return nil, err
		}
		if err = b.pin(equality); err != nil {
			return nil, err
		}
		*conjunction = append(*conjunction, equality)
	}
	return owner, nil
}

func (b *spacerRuleBuilder) groundVariables(expression ast.ASTHandle, variables *[]ast.ASTHandle, auxiliaries *[]ast.ASTHandle) error {
	used, err := ast.NewASTUsedVars(b.transformer.manager)
	if err != nil {
		return err
	}
	if err = used.Collect(expression); err != nil {
		return err
	}
	for uint32(len(*variables)) < used.MaxFoundVarIndexPlusOne() {
		*variables = append(*variables, ast.ASTHandle{})
	}
	for index := uint32(0); index < used.MaxFoundVarIndexPlusOne(); index++ {
		sort := used.Contains(index)
		if sort.IsNull() || !(*variables)[index].IsNull() {
			continue
		}
		value, err := b.fresh("aux", sort)
		if err != nil {
			return err
		}
		current, err := b.transformer.manager.GetDecl(value)
		if err != nil {
			return err
		}
		current, err = b.transformer.owner.NewPredicate(current)
		if err != nil {
			return err
		}
		value, err = b.transformer.manager.MkApp(current, nil)
		if err != nil {
			return err
		}
		if err = b.pin(value); err != nil {
			return err
		}
		(*variables)[index] = value
		*auxiliaries = append(*auxiliaries, value)
	}
	return nil
}

func (b *spacerRuleBuilder) substitute(expression ast.ASTHandle, variables []ast.ASTHandle) (ast.ASTHandle, error) {
	substitution, err := rewriter.NewVarSubstitution(b.transformer.manager, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer substitution.Close()
	out, err := b.transformer.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = substitution.Substitute(expression, variables, out); err != nil {
		return ast.ASTHandle{}, errors.Join(err, b.transformer.manager.DelRef(&out))
	}
	result, err := b.transformer.manager.RefGet(out)
	if err != nil {
		return ast.ASTHandle{}, errors.Join(err, b.transformer.manager.DelRef(&out))
	}
	b.references = append(b.references, out)
	return result, nil
}

func (b *spacerRuleBuilder) normalizeRule(source *base.Rule) (*spacerNormalizedRule, error) {
	owned, err := source.Retain()
	if err != nil {
		return nil, err
	}
	result := &spacerNormalizedRule{rule: owned}
	committed := false
	defer func() {
		if !committed {
			_ = owned.Close()
		}
	}()
	headDeclaration, err := owned.HeadDeclaration()
	if err != nil {
		return nil, err
	}
	head, err := b.transformer.Head()
	if err != nil || headDeclaration != head {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return nil, err
	}
	headApplication, err := owned.Head()
	if err != nil {
		return nil, err
	}
	variables := make([]ast.ASTHandle, 0)
	conjunction := make([]ast.ASTHandle, 0)
	if _, err = b.initializeAtom(headApplication, &variables, &conjunction, nil); err != nil {
		return nil, err
	}
	uninterpreted, err := owned.UninterpretedTailSize()
	if err != nil {
		return nil, err
	}
	tailSize, err := owned.TailSize()
	if err != nil {
		return nil, err
	}
	for index := 0; index < uninterpreted; index++ {
		tail, negated, err := owned.Tail(index)
		if err != nil {
			return nil, err
		}
		if negated {
			return nil, fmt.Errorf("muz/spacer: negated predicate tails are unsupported")
		}
		old := uint32(index)
		predecessor, err := b.initializeAtom(tail, &variables, &conjunction, &old)
		if err != nil {
			return nil, err
		}
		result.predecessors = append(result.predecessors, predecessor)
	}
	interpreted := make([]ast.ASTHandle, 0, tailSize-uninterpreted)
	for index := uninterpreted; index < tailSize; index++ {
		tail, _, err := owned.Tail(index)
		if err != nil {
			return nil, err
		}
		interpreted = append(interpreted, tail)
	}
	theory, err := b.transformer.manager.MkAndArray(interpreted)
	if err != nil {
		return nil, err
	}
	if err = b.groundVariables(theory, &variables, &result.auxiliaries); err != nil {
		return nil, err
	}
	theory, err = b.substitute(theory, variables)
	if err != nil {
		return nil, err
	}
	flattened, err := b.transformer.manager.FlattenAnd([]ast.ASTHandle{theory})
	if err != nil {
		return nil, err
	}
	conjunction = append(conjunction, flattened...)
	formula, err := b.transformer.manager.MkAndArray(conjunction)
	if err != nil {
		return nil, err
	}
	if err = b.pin(formula); err != nil {
		return nil, err
	}
	result.transition, err = b.rewrite(formula)
	if err != nil {
		return nil, err
	}
	if b.transformer.blastTermITE {
		result.transition, err = b.blastTermITE(result.transition)
		if err != nil {
			return nil, err
		}
		result.transition, err = b.rewrite(result.transition)
		if err != nil {
			return nil, err
		}
	}
	if result.initial = uninterpreted == 0; !result.initial {
		ground, err := b.transformer.manager.IsGround(result.transition)
		if err != nil || !ground {
			if err == nil {
				err = fmt.Errorf("muz/spacer: normalized non-initial transition is not ground")
			}
			return nil, err
		}
	}
	result.instances = append(result.instances, variables...)
	committed = true
	return result, nil
}

func (b *spacerRuleBuilder) tag(rule *spacerNormalizedRule, index, count int) error {
	boolean, err := b.transformer.manager.MkBoolSort()
	if err != nil {
		return err
	}
	name := b.transformer.name + "_dummy"
	if count != 1 {
		name = fmt.Sprintf("%s_tr%d", b.transformer.name, index)
	}
	rule.tag, err = b.named(name, boolean)
	return err
}

func (b *spacerRuleBuilder) buildSystem() (ast.ASTHandle, ast.ASTHandle, error) {
	live := make([]*spacerNormalizedRule, 0, len(b.rules))
	for _, rule := range b.rules {
		if !b.transformer.manager.IsFalse(rule.transition) {
			live = append(live, rule)
		}
	}
	if len(live) == 0 {
		b.transformer.allInitial = true
		initial, err := b.transformer.manager.MkTrue()
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		transition, err := b.transformer.manager.MkFalse()
		return initial, transition, err
	}
	extendLiteral, err := b.transformer.manager.RefGet(b.transformer.extendLiteral)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	negated, extension, err := b.transformer.manager.IsNot(extendLiteral)
	if err != nil || !negated {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	guarded := make([]ast.ASTHandle, 0, len(live)+1)
	selection := make([]ast.ASTHandle, 0, len(live)+1)
	selection = append(selection, extension)
	initialConditions := make([]ast.ASTHandle, 0, len(live))
	for index, rule := range live {
		if err = b.tag(rule, index, len(live)); err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		selection = append(selection, rule.tag)
		implication, err := b.transformer.manager.MkImplies(rule.tag, rule.transition)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		if err = b.pin(implication); err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		guarded = append(guarded, implication)
		if !rule.initial {
			disabled, err := b.transformer.manager.MkNot(rule.tag)
			if err != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, err
			}
			if err = b.pin(disabled); err != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, err
			}
			initialConditions = append(initialConditions, disabled)
		}
	}
	selected, err := b.transformer.manager.MkOrArray(selection)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if err = b.pin(selected); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	guarded = append(guarded, selected)
	transition, err := b.transformer.manager.MkAndArray(guarded)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if err = b.pin(transition); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	transition, err = b.rewrite(transition)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	initial, err := b.transformer.manager.MkAndArray(initialConditions)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if err = b.pin(initial); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	initial, err = b.rewrite(initial)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	b.transformer.allInitial = len(initialConditions) == 0
	return initial, transition, nil
}

// InitializeRules converts the rules headed by this transformer into the
// source Spacer tagged transition system. Each predicate declaration used by
// a rule atom must identify its headed transformer in transformers.
func (p *PredicateTransformer) InitializeRules(transformers map[ast.ASTHandle]*PredicateTransformer, rules []*base.Rule) (resultErr error) {
	if err := p.open(); err != nil {
		return err
	}
	if p.initialized || len(p.rules) != 0 {
		return ast.ErrASTDomain
	}
	head, err := p.Head()
	if err != nil || transformers[head] != p {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return err
	}
	config := p.owner.contexts[0].config
	theory, err := rewriter.NewTheoryRewriter(p.manager, config.Params, config.Globals, nil)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, theory.Close()) }()
	builder := &spacerRuleBuilder{transformer: p, all: transformers, theory: theory}
	committed := false
	defer func() {
		if committed {
			return
		}
		for index := len(builder.rules) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, builder.rules[index].rule.Close())
		}
		for index := len(builder.references) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&builder.references[index]))
		}
	}()
	for _, rule := range rules {
		normalized, err := builder.normalizeRule(rule)
		if err != nil {
			return err
		}
		builder.rules = append(builder.rules, normalized)
	}
	initial, transition, err := builder.buildSystem()
	if err != nil {
		return err
	}
	if err = p.InitializeSystem(initial, transition); err != nil {
		return err
	}
	p.ruleReferences = append(p.ruleReferences, builder.references...)
	p.rules = builder.rules
	for _, rule := range p.rules {
		for _, predecessor := range rule.predecessors {
			predecessor.addUse(p)
		}
	}
	committed = true
	return nil
}

func (p *PredicateTransformer) addUse(user *PredicateTransformer) {
	for _, prior := range p.uses {
		if prior == user {
			return
		}
	}
	p.uses = append(p.uses, user)
}

func (p *PredicateTransformer) SetQuantifiedLemmaPolicy(instantiate, retain bool) error {
	if err := p.open(); err != nil {
		return err
	}
	p.instantiateQuantified = instantiate
	p.useQuantifiedLemmas = retain
	return nil
}

func (p *PredicateTransformer) SetBlastTermITE(value bool) error {
	if err := p.open(); err != nil {
		return err
	}
	p.blastTermITE = value
	return nil
}

func (p *PredicateTransformer) makeAssumptions(head, formula ast.ASTHandle) (_ []ast.ASTRef, resultErr error) {
	var result []ast.ASTRef
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, releaseNormalizeRefs(p.manager, result))
		}
	}()
	for _, rule := range p.rules {
		if rule.tag.IsNull() {
			continue
		}
		for index, predecessor := range rule.predecessors {
			predecessorHead, err := predecessor.Head()
			if err != nil {
				return nil, err
			}
			if predecessorHead != head {
				continue
			}
			implication, err := p.manager.MkImplies(rule.tag, formula)
			if err != nil {
				return nil, err
			}
			converted, err := p.owner.FormulaNewToOld(implication, uint32(index), false)
			if err != nil {
				return nil, err
			}
			result = append(result, converted)
		}
	}
	return result, nil
}

func (p *PredicateTransformer) addLemmaFromChild(child *PredicateTransformer, lemma *Lemma, level uint32) (resultErr error) {
	if level != InfinityLevel {
		if err := p.EnsureLevel(level); err != nil {
			return err
		}
	}
	head, err := child.Head()
	if err != nil {
		return err
	}
	body, err := lemma.Body()
	if err != nil {
		return err
	}
	formulas, err := p.makeAssumptions(head, body)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, releaseNormalizeRefs(p.manager, formulas)) }()
	ground, err := p.manager.IsGround(body)
	if err != nil {
		return err
	}
	for _, formulaRef := range formulas {
		formula, err := p.manager.RefGet(formulaRef)
		if err != nil {
			return err
		}
		antecedent, err := p.manager.GetArg(formula, 0)
		if err != nil {
			return err
		}
		consequent, err := p.manager.GetArg(formula, 1)
		if err != nil {
			return err
		}
		var assertions []ast.ASTRef
		if p.instantiateQuantified {
			assertions, err = lemma.instancesOf(consequent)
			if err != nil {
				return err
			}
			for i := range assertions {
				instance, getErr := p.manager.RefGet(assertions[i])
				if getErr != nil {
					_ = releaseNormalizeRefs(p.manager, assertions)
					return getErr
				}
				guarded, makeErr := p.manager.MkImplies(antecedent, instance)
				if makeErr != nil {
					_ = releaseNormalizeRefs(p.manager, assertions)
					return makeErr
				}
				if makeErr = p.manager.SetRef(assertions[i], guarded); makeErr != nil {
					_ = releaseNormalizeRefs(p.manager, assertions)
					return makeErr
				}
			}
		}
		if ground || p.useQuantifiedLemmas {
			ref, refErr := p.manager.CopyRef(formulaRef)
			if refErr != nil {
				_ = releaseNormalizeRefs(p.manager, assertions)
				return refErr
			}
			assertions = append(assertions, ref)
		}
		if len(assertions) == 0 {
			return ast.ErrASTDomain
		}
		for i := range assertions {
			assertion, getErr := p.manager.RefGet(assertions[i])
			if getErr != nil {
				_ = releaseNormalizeRefs(p.manager, assertions)
				return getErr
			}
			if level == InfinityLevel {
				getErr = p.solver.Assert(assertion)
			} else {
				getErr = p.solver.AssertAtLevel(assertion, level)
			}
			if getErr != nil {
				_ = releaseNormalizeRefs(p.manager, assertions)
				return getErr
			}
		}
		if err = releaseNormalizeRefs(p.manager, assertions); err != nil {
			return err
		}
	}
	return nil
}

func (p *PredicateTransformer) Rules() ([]*base.Rule, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	result := make([]*base.Rule, 0, len(p.rules))
	for _, rule := range p.rules {
		owned, err := rule.rule.Retain()
		if err != nil {
			for _, prior := range result {
				_ = prior.Close()
			}
			return nil, err
		}
		result = append(result, owned)
	}
	return result, nil
}

func (p *PredicateTransformer) InitialState() (ast.ASTHandle, error) {
	if err := p.open(); err != nil || !p.initialized {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, err
	}
	return p.manager.RefGet(p.initial)
}

func (p *PredicateTransformer) Transition() (ast.ASTHandle, error) {
	if err := p.open(); err != nil || !p.initialized {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, err
	}
	return p.manager.RefGet(p.transition)
}

func (p *PredicateTransformer) AllInitial() (bool, error) {
	if err := p.open(); err != nil || !p.initialized {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return false, err
	}
	return p.allInitial, nil
}
