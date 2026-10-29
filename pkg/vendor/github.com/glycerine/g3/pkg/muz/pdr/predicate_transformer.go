package pdr

// Source: the retained predicate-transformer signature, frame-property,
// reachability and propagation core in src/muz/pdr/pdr_context.h and
// src/muz/pdr/pdr_context.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

var ErrPredicateTransformerOwner = errors.New("muz/pdr: invalid or closed predicate transformer")

const InfinityLevel uint32 = ^uint32(0)

type PredicateTransformerStatistics struct {
	Propagations uint32
	Properties   uint32
}

// PredicateTransformer joins one recursive predicate's PDR state signature,
// frame lemmas, proposition solver, and reachable-state cache. Rule
// normalization populates InitialState and Transition through InitializeSystem.
type PredicateTransformer struct {
	owner         *Manager
	manager       *ast.ASTManager
	head          ast.ASTHandle
	signature     []ast.ASTHandle
	solver        *PropSolver
	reachable     *ReachableCache
	initialState  ast.ASTHandle
	transition    ast.ASTHandle
	levels        [][]ast.ASTHandle
	invariants    []ast.ASTHandle
	propertyLevel map[ast.ASTHandle]uint32
	rules         []*normalizedRule
	uses          []*PredicateTransformer
	references    []ast.ASTRef
	propagations  uint32
	initialized   bool
	rulesReady    bool
	closed        bool
}

func NewPredicateTransformer(owner *Manager, head ast.ASTHandle, cacheMode ReachableCacheMode) (_ *PredicateTransformer, resultErr error) {
	if owner == nil {
		return nil, ErrPredicateTransformerOwner
	}
	if err := owner.open(); err != nil {
		return nil, err
	}
	manager := owner.manager
	kind, err := manager.GetKind(head)
	if err != nil || kind != ast.ASTFuncDecl {
		if err != nil {
			return nil, err
		}
		return nil, ast.ErrASTDomain
	}
	rangeSort, err := manager.GetRange(head)
	if err != nil {
		return nil, err
	}
	boolean, err := manager.MkBoolSort()
	if err != nil || rangeSort != boolean {
		if err != nil {
			return nil, err
		}
		return nil, ast.ErrASTDomain
	}
	transformer := &PredicateTransformer{
		owner: owner, manager: manager, head: head,
		propertyLevel: make(map[ast.ASTHandle]uint32),
	}
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, transformer.Close())
		}
	}()
	if err = transformer.pin(head); err != nil {
		return nil, err
	}
	name, err := transformer.headName()
	if err != nil {
		return nil, err
	}
	transformer.solver, err = owner.NewPropSolver(name)
	if err != nil {
		return nil, err
	}
	transformer.reachable, err = owner.NewReachableCache(cacheMode)
	if err != nil {
		return nil, err
	}
	if err = transformer.initializeSignature(name); err != nil {
		return nil, err
	}
	committed = true
	return transformer, nil
}

func (p *PredicateTransformer) open() error {
	if p == nil || p.closed || p.owner == nil || p.manager == nil || p.solver == nil || p.reachable == nil {
		return ErrPredicateTransformerOwner
	}
	return p.owner.open()
}

func (p *PredicateTransformer) pin(handle ast.ASTHandle) error {
	reference, err := p.manager.NewRef(handle)
	if err == nil {
		p.references = append(p.references, reference)
	}
	return err
}

func (p *PredicateTransformer) headName() (string, error) {
	name, err := p.manager.GetASTName(p.head)
	if err != nil {
		return "", err
	}
	if text, present, x := p.manager.Symbols.BareString(name); x != nil {
		return "", x
	} else if present {
		return text, nil
	}
	return p.manager.Symbols.Display(name)
}

func (p *PredicateTransformer) initializeSignature(headName string) error {
	arity, err := p.manager.GetArity(p.head)
	if err != nil {
		return err
	}
	p.signature = make([]ast.ASTHandle, arity)
	for index := uint32(0); index < arity; index++ {
		sort, x := p.manager.GetDomain(p.head, index)
		if x != nil {
			return x
		}
		name, x := p.manager.Symbols.Intern(fmt.Sprintf("%s_%d", headName, index))
		if x != nil {
			return x
		}
		prefix, x := p.manager.MkFuncDecl(name, nil, sort, ast.ASTFuncDeclInfo{})
		if x != nil {
			return x
		}
		p.signature[index], x = p.owner.OldPredicate(prefix, 0)
		if x != nil {
			return x
		}
	}
	return nil
}

func (p *PredicateTransformer) Head() (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return p.head, nil
}

// Signature returns the old-state-zero declarations used for the predicate's
// arguments. The returned slice is independent and its handles are borrowed.
func (p *PredicateTransformer) Signature() ([]ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), p.signature...), nil
}

// InitializeSystem installs the normalized initial-state and transition
// formulas produced from this predicate's rules.
func (p *PredicateTransformer) InitializeSystem(initialState, transition ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	if p.initialized {
		return ast.ErrASTDomain
	}
	initialRef, err := p.manager.NewRef(initialState)
	if err != nil {
		return err
	}
	transitionRef, err := p.manager.NewRef(transition)
	if err != nil {
		return errors.Join(err, p.manager.DelRef(&initialRef))
	}
	if err = p.solver.AddFormula(transition); err != nil {
		return errors.Join(err, p.manager.DelRef(&transitionRef), p.manager.DelRef(&initialRef))
	}
	if err = p.solver.AddLevelFormula(initialState, 0); err != nil {
		return errors.Join(err, p.manager.DelRef(&transitionRef), p.manager.DelRef(&initialRef))
	}
	if err = p.reachable.AddInit(initialState); err != nil {
		return errors.Join(err, p.manager.DelRef(&transitionRef), p.manager.DelRef(&initialRef))
	}
	p.references = append(p.references, initialRef, transitionRef)
	p.initialState = initialState
	p.transition = transition
	p.initialized = true
	return nil
}

func (p *PredicateTransformer) InitialState() (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if !p.initialized {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return p.initialState, nil
}

func (p *PredicateTransformer) Transition() (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if !p.initialized {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return p.transition, nil
}

func (p *PredicateTransformer) EnsureLevel(level uint32) error {
	if err := p.open(); err != nil {
		return err
	}
	if level == InfinityLevel {
		return nil
	}
	for uint32(len(p.levels)) <= level {
		if err := p.solver.EnsureLevel(uint32(len(p.levels))); err != nil {
			return err
		}
		p.levels = append(p.levels, nil)
	}
	return nil
}

func (p *PredicateTransformer) addPropertyOne(lemma ast.ASTHandle, level uint32) (bool, error) {
	if level == InfinityLevel {
		if old, found := p.propertyLevel[lemma]; found && old == InfinityLevel {
			return false, nil
		}
		reference, err := p.manager.NewRef(lemma)
		if err != nil {
			return false, err
		}
		if err = p.solver.AddFormula(lemma); err != nil {
			return false, errors.Join(err, p.manager.DelRef(&reference))
		}
		p.references = append(p.references, reference)
		p.invariants = append(p.invariants, lemma)
		p.propertyLevel[lemma] = level
		return true, nil
	}
	if err := p.EnsureLevel(level); err != nil {
		return false, err
	}
	if old, found := p.propertyLevel[lemma]; found && old >= level {
		return false, nil
	}
	reference, err := p.manager.NewRef(lemma)
	if err != nil {
		return false, err
	}
	if err = p.solver.AddLevelFormula(lemma, level); err != nil {
		return false, errors.Join(err, p.manager.DelRef(&reference))
	}
	p.references = append(p.references, reference)
	p.levels[level] = append(p.levels[level], lemma)
	p.propertyLevel[lemma] = level
	return true, nil
}

// AddProperty splits conjunctions into frame lemmas and reports whether at
// least one conjunct was newly installed or moved to a stronger frame.
func (p *PredicateTransformer) AddProperty(lemma ast.ASTHandle, level uint32) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	lemmas, err := p.manager.FlattenAnd([]ast.ASTHandle{lemma})
	if err != nil {
		return false, err
	}
	changed := false
	for _, conjunct := range lemmas {
		added, x := p.addPropertyOne(conjunct, level)
		if x != nil {
			return changed, x
		}
		changed = changed || added
		if added {
			target := level
			if target != InfinityLevel {
				target++
			}
			for _, user := range p.uses {
				if x = user.addChildProperty(p, conjunct, target); x != nil {
					return changed, x
				}
			}
		}
	}
	return changed, nil
}

// NumLevels returns the number of finite PDR frames currently retained for
// this predicate. The invariant frame at InfinityLevel is not included.
func (p *PredicateTransformer) NumLevels() (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	return uint32(len(p.levels)), nil
}

// AddCover installs a property expressed over the predicate's bound
// variables. Level -1 denotes the invariant frame.
func (p *PredicateTransformer) AddCover(level int, property ast.ASTHandle) (resultErr error) {
	if err := p.open(); err != nil {
		return err
	}
	if level < -1 {
		return ast.ErrASTDomain
	}
	arguments := make([]ast.ASTHandle, len(p.signature))
	for index, declaration := range p.signature {
		stateDeclaration, err := p.owner.OldToNew(declaration, 0)
		if err != nil {
			return err
		}
		arguments[index], err = p.manager.MkApp(stateDeclaration, nil)
		if err != nil {
			return err
		}
	}
	substitution, err := rewriter.NewVarSubstitution(p.manager, false)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, substitution.Close()) }()
	converted, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&converted)) }()
	if err = substitution.Substitute(property, arguments, converted); err != nil {
		return err
	}
	handle, err := p.manager.RefGet(converted)
	if err != nil {
		return err
	}
	target := InfinityLevel
	if level >= 0 {
		target = uint32(level)
	}
	_, err = p.AddProperty(handle, target)
	return err
}

// CoverDelta returns an owning reference to exactly one finite frame, or to
// the invariant frame for level -1, translated back to bound variables.
func (p *PredicateTransformer) CoverDelta(level int) (result ast.ASTRef, resultErr error) {
	if err := p.open(); err != nil {
		return result, err
	}
	defer func() {
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	var formula ast.ASTHandle
	var err error
	switch {
	case level == -1:
		formula, err = p.manager.MkAndArray(p.invariants)
	case level >= 0 && level < len(p.levels):
		formula, err = p.manager.MkAndArray(p.levels[level])
	default:
		formula, err = p.manager.MkTrue()
	}
	if err != nil {
		return result, err
	}
	substitution, err := p.manager.NewExprSubstitution()
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelExprSubstitution(&substitution)) }()
	for index, declaration := range p.signature {
		stateDeclaration, x := p.owner.OldToNew(declaration, 0)
		if x != nil {
			return result, x
		}
		constant, x := p.manager.MkApp(stateDeclaration, nil)
		if x != nil {
			return result, x
		}
		sort, x := p.manager.GetRange(declaration)
		if x != nil {
			return result, x
		}
		variable, x := p.manager.MkVar(uint32(index), sort)
		if x != nil {
			return result, x
		}
		if x = p.manager.ExprSubstitutionInsert(substitution, constant, variable, ast.ASTHandle{}, ast.ASTDependency{}); x != nil {
			return result, x
		}
	}
	replacer, err := rewriter.NewDefaultExprReplacer(p.manager)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, replacer.Close()) }()
	if err = replacer.SetSubstitution(&substitution); err != nil {
		return result, err
	}
	result, err = p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	if err = replacer.Rewrite(formula, result); err != nil {
		return ast.ASTRef{}, errors.Join(err, p.manager.DelRef(&result))
	}
	return result, nil
}

func (p *PredicateTransformer) Formulas(level uint32, addAxioms bool) (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	formulas := make([]ast.ASTHandle, 0, len(p.invariants)+2)
	if addAxioms {
		if !p.initialized {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		background, err := p.owner.Background()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		formulas = append(formulas, background)
		if level == 0 {
			formulas = append(formulas, p.initialState)
		} else {
			formulas = append(formulas, p.transition)
		}
	}
	formulas = append(formulas, p.invariants...)
	if level != InfinityLevel {
		for index := level; index < uint32(len(p.levels)); index++ {
			formulas = append(formulas, p.levels[index]...)
		}
	}
	return p.manager.MkAndArray(formulas)
}

func (p *PredicateTransformer) CheckReachable(level uint32, state ast.ASTHandle, wantCore, wantModel bool) (PropCheckResult, error) {
	if err := p.open(); err != nil {
		return PropCheckResult{}, err
	}
	if level == InfinityLevel {
		return PropCheckResult{}, ast.ErrASTDomain
	}
	if err := p.EnsureLevel(level); err != nil {
		return PropCheckResult{}, err
	}
	return p.solver.CheckAtLevel(level, []ast.ASTHandle{state}, wantCore, wantModel)
}

// IsInvariantAtLevel tests whether the current frames refute not(property).
func (p *PredicateTransformer) IsInvariantAtLevel(level uint32, property ast.ASTHandle, wantCore bool) (bool, bool, []ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return false, false, nil, err
	}
	if level == InfinityLevel {
		return false, false, nil, ast.ErrASTDomain
	}
	negated, err := p.manager.MkNotSimplified(property)
	if err != nil {
		return false, false, nil, err
	}
	result, err := p.CheckReachable(level, negated, wantCore, false)
	return result.Status == lbool.False, result.AssumesLevel, result.Core, err
}

func (p *PredicateTransformer) IsCachedReachable(state ast.ASTHandle) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	return p.reachable.IsReachable(state)
}

func (p *PredicateTransformer) CloseState(state ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	return p.reachable.AddReachable(state)
}

// PropagateToNextLevel tries every lemma currently assigned to sourceLevel.
// It returns true when the source frame becomes empty.
func (p *PredicateTransformer) PropagateToNextLevel(sourceLevel uint32) (bool, error) {
	if sourceLevel == InfinityLevel || sourceLevel == InfinityLevel-1 {
		return false, ast.ErrASTDomain
	}
	if err := p.EnsureLevel(sourceLevel + 2); err != nil {
		return false, err
	}
	source := p.levels[sourceLevel]
	for index := 0; index < len(source); {
		lemma := source[index]
		stored, found := p.propertyLevel[lemma]
		if !found {
			return false, ast.ErrASTDomain
		}
		if stored > sourceLevel {
			source[index] = source[len(source)-1]
			source = source[:len(source)-1]
			continue
		}
		invariant, assumesLevel, _, err := p.IsInvariantAtLevel(sourceLevel+1, lemma, false)
		if err != nil {
			return false, err
		}
		if !invariant {
			index++
			continue
		}
		target := InfinityLevel
		if assumesLevel {
			target = sourceLevel + 1
		}
		if _, err = p.AddProperty(lemma, target); err != nil {
			return false, err
		}
		source[index] = source[len(source)-1]
		source = source[:len(source)-1]
		p.propagations++
	}
	p.levels[sourceLevel] = source
	return len(source) == 0, nil
}

func (p *PredicateTransformer) PropagateToInfinity(level uint32) error {
	if level == InfinityLevel {
		return ast.ErrASTDomain
	}
	formula, err := p.Formulas(level, false)
	if err != nil {
		return err
	}
	if _, err = p.AddProperty(formula, InfinityLevel); err != nil {
		return err
	}
	for index := level; index < uint32(len(p.levels)); index++ {
		p.levels[index] = nil
	}
	return nil
}

func (p *PredicateTransformer) Statistics() (PredicateTransformerStatistics, error) {
	if err := p.open(); err != nil {
		return PredicateTransformerStatistics{}, err
	}
	properties := uint32(len(p.invariants))
	for _, level := range p.levels {
		properties += uint32(len(level))
	}
	return PredicateTransformerStatistics{Propagations: p.propagations, Properties: properties}, nil
}

func (p *PredicateTransformer) CollectStatistics(destination *statistics.Statistics) error {
	if destination == nil {
		return ErrPredicateTransformerOwner
	}
	stats, err := p.Statistics()
	if err != nil {
		return err
	}
	if err = p.reachable.CollectStatistics(destination); err != nil {
		return err
	}
	destination.UpdateUint("PDR num propagations", stats.Propagations)
	destination.UpdateUint("PDR num properties", stats.Properties)
	return nil
}

func (p *PredicateTransformer) ResetStatistics() error {
	if err := p.open(); err != nil {
		return err
	}
	p.propagations = 0
	return p.reachable.ResetStatistics()
}

func (p *PredicateTransformer) Close() (result error) {
	if p == nil || p.manager == nil {
		return ErrPredicateTransformerOwner
	}
	if p.closed {
		return nil
	}
	if p.reachable != nil {
		result = errors.Join(result, p.reachable.Close())
	}
	if p.solver != nil {
		result = errors.Join(result, p.solver.Close())
	}
	for index := len(p.rules) - 1; index >= 0; index-- {
		if p.rules[index] != nil && p.rules[index].rule != nil {
			result = errors.Join(result, p.rules[index].rule.Close())
		}
	}
	for index := len(p.references) - 1; index >= 0; index-- {
		result = errors.Join(result, p.manager.DelRef(&p.references[index]))
	}
	if result == nil {
		p.owner = nil
		p.manager = nil
		p.solver = nil
		p.reachable = nil
		p.signature = nil
		p.levels = nil
		p.invariants = nil
		p.propertyLevel = nil
		p.rules = nil
		p.uses = nil
		p.references = nil
		p.closed = true
	}
	return result
}
