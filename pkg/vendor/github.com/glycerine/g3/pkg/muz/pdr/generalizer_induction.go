package pdr

// Source: core_induction_generalizer::imp and
// core_induction_generalizer::operator() in src/muz/pdr/pdr_generalizers.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

const inductionGeneralizerDepth uint32 = 2

type inductionGeneralizerBuilder struct {
	owner      *Manager
	manager    *ast.ASTManager
	substitute *rewriter.VarSubstitution
	abstract   *ast.ExprAbstractor
	references []ast.ASTRef
	frameRefs  []ast.ASTRef
}

func newInductionGeneralizerBuilder(transformer *PredicateTransformer) (_ *inductionGeneralizerBuilder, resultErr error) {
	if transformer == nil {
		return nil, ErrPredicateTransformerOwner
	}
	if err := transformer.open(); err != nil {
		return nil, err
	}
	builder := &inductionGeneralizerBuilder{owner: transformer.owner, manager: transformer.manager}
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, builder.Close())
		}
	}()
	var err error
	builder.substitute, err = rewriter.NewVarSubstitution(builder.manager, false)
	if err != nil {
		return nil, err
	}
	builder.abstract, err = ast.NewExprAbstractor(builder.manager)
	if err != nil {
		return nil, err
	}
	committed = true
	return builder, nil
}

func (b *inductionGeneralizerBuilder) pin(handle ast.ASTHandle) error {
	reference, err := b.manager.NewRef(handle)
	if err == nil {
		b.references = append(b.references, reference)
	}
	return err
}

func (b *inductionGeneralizerBuilder) keep(handle ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = b.pin(handle); err != nil {
		return ast.ASTHandle{}, err
	}
	return handle, nil
}

func (b *inductionGeneralizerBuilder) predicate(level uint32, transformer *PredicateTransformer) (ast.ASTHandle, error) {
	name, err := transformer.headName()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	symbol, err := b.manager.Symbols.Intern(fmt.Sprintf("%s_%d", name, level))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arity, err := b.manager.GetArity(transformer.head)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	domain := make([]ast.ASTHandle, arity)
	for index := uint32(0); index < arity; index++ {
		domain[index], err = b.manager.GetDomain(transformer.head, index)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	rangeSort, err := b.manager.GetRange(transformer.head)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return b.keep(b.manager.MkFuncDecl(symbol, domain, rangeSort, ast.ASTFuncDeclInfo{}))
}

func (b *inductionGeneralizerBuilder) representatives(transformer *PredicateTransformer) ([]ast.ASTHandle, error) {
	signature, err := transformer.Signature()
	if err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, len(signature))
	for index, declaration := range signature {
		declaration, err = b.owner.OldToNew(declaration, 0)
		if err != nil {
			return nil, err
		}
		result[index], err = b.keep(b.manager.MkApp(declaration, nil))
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func inductionApplicationArguments(manager *ast.ASTManager, application ast.ASTHandle) ([]ast.ASTHandle, error) {
	count, err := manager.GetNumArgs(application)
	if err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, count)
	for index := uint32(0); index < count; index++ {
		result[index], err = manager.GetArg(application, index)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (b *inductionGeneralizerBuilder) bindHead(representatives []ast.ASTHandle, formula ast.ASTHandle) (ast.ASTHandle, error) {
	if len(representatives) == 0 {
		return formula, nil
	}
	result, err := b.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = b.manager.DelRef(&result)
		}
	}()
	if err = b.abstract.Abstract(0, representatives, formula, result); err != nil {
		return ast.ASTHandle{}, err
	}
	body, err := b.manager.RefGet(result)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sorts := make([]ast.ASTHandle, len(representatives))
	names := make([]usymbol.Symbol, len(representatives))
	for index := range representatives {
		reversed := len(representatives) - index - 1
		sorts[index], err = b.manager.GetSort(representatives[reversed])
		if err != nil {
			return ast.ASTHandle{}, err
		}
		names[index] = usymbol.Numerical(uint32(reversed))
	}
	quantified, err := b.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: body})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = b.manager.SetRef(result, quantified); err != nil {
		return ast.ASTHandle{}, err
	}
	b.references = append(b.references, result)
	committed = true
	return quantified, nil
}

func (b *inductionGeneralizerBuilder) bindFreeVariables(formula ast.ASTHandle) (ast.ASTHandle, error) {
	used, err := ast.NewASTUsedVars(b.manager)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = used.Collect(formula); err != nil {
		return ast.ASTHandle{}, err
	}
	count := used.MaxFoundVarIndexPlusOne()
	if count == 0 {
		return formula, nil
	}
	boolean, err := b.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sorts := make([]ast.ASTHandle, count)
	names := make([]usymbol.Symbol, count)
	for index := uint32(0); index < count; index++ {
		sort := used.Contains(index)
		if sort.IsNull() {
			sort = boolean
		}
		reversed := count - index - 1
		sorts[reversed] = sort
		names[index] = usymbol.Numerical(reversed)
	}
	return b.keep(b.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: sorts, Names: names, Body: formula}))
}

func (b *inductionGeneralizerBuilder) transitionRule(transformer *PredicateTransformer, level uint32, normalized *normalizedRule, representatives []ast.ASTHandle) (ast.ASTHandle, error) {
	rule := normalized.rule
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if level == 0 && uninterpreted != 0 {
		return b.keep(b.manager.MkFalse())
	}
	head, err := rule.Head()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	headArguments, err := inductionApplicationArguments(b.manager, head)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if len(headArguments) != len(representatives) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	conjunction := make([]ast.ASTHandle, 0)
	bindings := make([]ast.ASTHandle, 0)
	for index, argument := range headArguments {
		kind, x := b.manager.GetKind(argument)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		if kind != ast.ASTVar {
			equality, x := b.keep(b.manager.MkEq(argument, representatives[index]))
			if x != nil {
				return ast.ASTHandle{}, x
			}
			conjunction = append(conjunction, equality)
			continue
		}
		variable, x := b.manager.GetVarIndex(argument)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		for uint32(len(bindings)) <= variable {
			bindings = append(bindings, ast.ASTHandle{})
		}
		if !bindings[variable].IsNull() {
			equality, x := b.keep(b.manager.MkEq(bindings[variable], representatives[index]))
			if x != nil {
				return ast.ASTHandle{}, x
			}
			conjunction = append(conjunction, equality)
		} else {
			bindings[variable] = representatives[index]
		}
	}
	for index := 0; index < uninterpreted; index++ {
		tail, _, x := rule.Tail(index)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		arguments, x := inductionApplicationArguments(b.manager, tail)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		predicate, x := b.predicate(level-1, normalized.predecessors[index])
		if x != nil {
			return ast.ASTHandle{}, x
		}
		application, x := b.keep(b.manager.MkApp(predicate, arguments))
		if x != nil {
			return ast.ASTHandle{}, x
		}
		conjunction = append(conjunction, application)
	}
	tailSize, err := rule.TailSize()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for index := uninterpreted; index < tailSize; index++ {
		tail, _, x := rule.Tail(index)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		conjunction = append(conjunction, tail)
	}
	formula, err := b.keep(b.manager.MkAndArray(conjunction))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if len(bindings) != 0 {
		result, x := b.manager.NewRef(ast.ASTHandle{})
		if x != nil {
			return ast.ASTHandle{}, x
		}
		if x = b.substitute.Substitute(formula, bindings, result); x != nil {
			return ast.ASTHandle{}, errors.Join(x, b.manager.DelRef(&result))
		}
		formula, x = b.manager.RefGet(result)
		if x != nil {
			return ast.ASTHandle{}, errors.Join(x, b.manager.DelRef(&result))
		}
		b.references = append(b.references, result)
	}
	return b.bindFreeVariables(formula)
}

func (b *inductionGeneralizerBuilder) transitionAxiom(transformer *PredicateTransformer, level uint32) (ast.ASTHandle, error) {
	representatives, err := b.representatives(transformer)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	transitions := make([]ast.ASTHandle, 0, len(transformer.rules))
	for _, rule := range transformer.rules {
		transition, x := b.transitionRule(transformer, level, rule, representatives)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		transitions = append(transitions, transition)
	}
	body, err := b.keep(b.manager.MkOrArray(transitions))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	predicate, err := b.predicate(level, transformer)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	application, err := b.keep(b.manager.MkApp(predicate, representatives))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	formula, err := b.keep(b.manager.MkIff(application, body))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return b.bindHead(representatives, formula)
}

func (b *inductionGeneralizerBuilder) predicateProperty(level uint32, transformer *PredicateTransformer, property ast.ASTHandle) (ast.ASTHandle, error) {
	representatives, err := b.representatives(transformer)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	predicate, err := b.predicate(level, transformer)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	application, err := b.keep(b.manager.MkApp(predicate, representatives))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	formula, err := b.keep(b.manager.MkImplies(application, property))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return b.bindHead(representatives, formula)
}

func (b *inductionGeneralizerBuilder) blockedTransition(transformer *PredicateTransformer, level uint32) (ast.ASTHandle, error) {
	if level == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	representatives, err := b.representatives(transformer)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	blocked := make([]ast.ASTHandle, 0, len(transformer.rules))
	for _, rule := range transformer.rules {
		transition, x := b.transitionRule(transformer, level, rule, representatives)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		negated, x := b.keep(b.manager.MkNot(transition))
		if x != nil {
			return ast.ASTHandle{}, x
		}
		blocked = append(blocked, negated)
	}
	return b.keep(b.manager.MkAndArray(blocked))
}

type inductionGeneralizerEntry struct {
	transformer *PredicateTransformer
	level       uint32
}

func (b *inductionGeneralizerBuilder) goal(transformer *PredicateTransformer, level, depth uint32) (ast.ASTHandle, error) {
	if level == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	blocked, err := b.blockedTransition(transformer, level)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	property, err := b.predicateProperty(level, transformer, blocked)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negated, err := b.keep(b.manager.MkNot(property))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	conjunction := []ast.ASTHandle{negated}
	queue := []inductionGeneralizerEntry{{transformer: transformer, level: level}}
	// The source uses unsigned level-depth. At level < depth that underflows,
	// so its release build adds no induction hypotheses.
	if level >= depth {
		for hypothesis := level - depth; hypothesis < level; hypothesis++ {
			if hypothesis == 0 {
				continue
			}
			blocked, x := b.blockedTransition(transformer, hypothesis)
			if x != nil {
				return ast.ASTHandle{}, x
			}
			property, x := b.predicateProperty(hypothesis, transformer, blocked)
			if x != nil {
				return ast.ASTHandle{}, x
			}
			conjunction = append(conjunction, property)
			queue = append(queue, inductionGeneralizerEntry{transformer: transformer, level: hypothesis})
		}
	}
	for cursor := 0; cursor < len(queue); cursor++ {
		entry := queue[cursor]
		axiom, x := b.transitionAxiom(entry.transformer, entry.level)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		conjunction = append(conjunction, axiom)
		formulaRef, x := entry.transformer.FormulasRef(entry.level, true)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		b.frameRefs = append(b.frameRefs, formulaRef)
		formulas, x := b.manager.RefGet(formulaRef)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		frameProperty, x := b.predicateProperty(entry.level, entry.transformer, formulas)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		conjunction = append(conjunction, frameProperty)
		if entry.level == 0 || entry.level+depth < level {
			continue
		}
		for _, rule := range entry.transformer.rules {
			for _, predecessor := range rule.predecessors {
				wanted := inductionGeneralizerEntry{transformer: predecessor, level: entry.level - 1}
				found := false
				for _, prior := range queue {
					if prior == wanted {
						found = true
						break
					}
				}
				if !found {
					queue = append(queue, wanted)
				}
			}
		}
	}
	return b.keep(b.manager.MkAndArray(conjunction))
}

func (b *inductionGeneralizerBuilder) Close() (result error) {
	if b == nil {
		return nil
	}
	if b.abstract != nil {
		result = errors.Join(result, b.abstract.Close())
		b.abstract = nil
	}
	if b.substitute != nil {
		result = errors.Join(result, b.substitute.Close())
		b.substitute = nil
	}
	for i := len(b.frameRefs) - 1; i >= 0; i-- {
		result = errors.Join(result, b.owner.releaseNotAndRef(&b.frameRefs[i]))
	}
	b.frameRefs = nil
	result = errors.Join(result, closeASTRefs(b.manager, b.references))
	b.references = nil
	b.owner = nil
	b.manager = nil
	return result
}

// generalizeInductionCore instantiates the source reachability-induction
// schema at the blocked node's parent. Its result independently owns all
// literals whether the induction succeeds or falls back to the input core.
func generalizeInductionCore(node *ModelNode, input generalizedCore) (result generalizedCore, resultErr error) {
	if node == nil {
		return generalizedCore{}, ErrModelSearchOwner
	}
	if err := node.open(); err != nil {
		return generalizedCore{}, err
	}
	parent := node.parent
	if parent == nil {
		return pinGeneralizedCore(node.transformer.manager, input.literals, input.usesLevel)
	}
	if err := parent.open(); err != nil {
		return generalizedCore{}, err
	}
	builder, err := newInductionGeneralizerBuilder(parent.transformer)
	if err != nil {
		return generalizedCore{}, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, builder.Close())
		if resultErr != nil {
			resultErr = errors.Join(resultErr, closeASTRefs(node.transformer.manager, result.references))
			result = generalizedCore{}
		}
	}()
	goal, err := builder.goal(parent.transformer, parent.level, inductionGeneralizerDepth)
	if err != nil {
		return generalizedCore{}, err
	}
	context, err := parent.transformer.owner.contexts.Fresh()
	if err != nil {
		return generalizedCore{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, context.Close()) }()
	if err = context.Assert(goal); err != nil {
		return generalizedCore{}, err
	}
	status, err := context.Check(nil)
	if err != nil {
		return generalizedCore{}, err
	}
	if status != lbool.False {
		return pinGeneralizedCore(node.transformer.manager, input.literals, input.usesLevel)
	}
	blocked, err := builder.blockedTransition(parent.transformer, parent.level)
	if err != nil {
		return generalizedCore{}, err
	}
	replacement, err := builder.keep(builder.manager.MkNot(blocked))
	if err != nil {
		return generalizedCore{}, err
	}
	return pinGeneralizedCore(node.transformer.manager, []ast.ASTHandle{replacement}, true)
}
