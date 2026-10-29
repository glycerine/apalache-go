package spacer

// Source: the initialized solver, lemma/frame, formula, and invariant paths in
// src/muz/spacer/spacer_context.cpp pred_transformer.

import (
	"errors"
	"fmt"
	"time"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

type PredicateTransformer struct {
	owner                    *Manager
	manager                  *ast.ASTManager
	solver                   *PropSolver
	reachSolver              *VirtualSolver
	frames                   *Frames
	obligations              *ProofObligationStore
	name                     string
	head                     ast.ASTRef
	signature                []ast.ASTRef
	extendLiteral            ast.ASTRef
	initial                  ast.ASTRef
	transition               ast.ASTRef
	initialized              bool
	allInitial               bool
	propagations             uint64
	invariants               uint64
	boolGeneralizations      uint64
	boolGeneralizerFailures  uint64
	boolGeneralizerTime      time.Duration
	simplifyProofObligations bool
	useEqualityClasses       bool
	instantiateQuantified    bool
	useQuantifiedLemmas      bool
	blastTermITE             bool
	useInductiveGeneralizer  bool
	useArrayEqGeneralizer    bool
	useEqClassGeneralizer    bool
	lemmaSanityCheck         bool
	useLemmaAsCTI            bool
	useDerivations           bool
	reverseChildren          bool
	eliminateAuxiliaries     bool
	reachDNF                 bool
	groundCTI                bool
	weakAbstraction          bool
	rules                    []*spacerNormalizedRule
	ruleReferences           []ast.ASTRef
	uses                     []*PredicateTransformer
	reachFacts               []*ReachFact
	reachCaseVariables       []ast.ASTRef
	initialReachFacts        uint32
	reachAsInitial           bool
	initializeReachFacts     bool
	closed                   bool
}

func (p *PredicateTransformer) SetProofObligationNormalization(simplifyBounds, factorEqualities bool) error {
	if err := p.open(); err != nil {
		return err
	}
	p.simplifyProofObligations = simplifyBounds
	p.useEqualityClasses = factorEqualities
	return nil
}

func (p *PredicateTransformer) SetProofObligationReuse(reuse bool) error {
	if err := p.open(); err != nil {
		return err
	}
	return p.obligations.SetReuse(reuse)
}

func (p *PredicateTransformer) NewProofObligation(parent *ProofObligation, level, depth uint32, post ast.ASTHandle, bindings []ast.ASTHandle) (*ProofObligation, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	return p.obligations.New(parent, level, depth, post, bindings)
}

func newPredicateTransformer(owner *Manager, name string, config PropSolverConfig) (_ *PredicateTransformer, resultErr error) {
	if owner == nil || owner.open() != nil {
		return nil, ErrManagerOwner
	}
	p := &PredicateTransformer{owner: owner, manager: owner.manager, name: name, instantiateQuantified: true, useQuantifiedLemmas: true, blastTermITE: true, useInductiveGeneralizer: config.UseInductiveGeneralizer, useArrayEqGeneralizer: config.UseArrayEqGeneralizer, useEqClassGeneralizer: config.UseEqClassGeneralizer, lemmaSanityCheck: config.LemmaSanityCheck, useLemmaAsCTI: config.UseLemmaAsCTI, useDerivations: true, eliminateAuxiliaries: true, reachDNF: true, groundCTI: true, weakAbstraction: true, reachAsInitial: true, initializeReachFacts: true}
	if config.Itp.Params == nil && config.Itp.Globals == nil {
		config.Itp.Params = owner.contexts[0].config.Params
		config.Itp.Globals = owner.contexts[0].config.Globals
	}
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, p.Close())
		}
	}()
	var err error
	if p.solver, err = owner.NewPropSolver(name, config); err != nil {
		return nil, err
	}
	if p.reachSolver, err = owner.Fresh(2); err != nil {
		return nil, err
	}
	if p.frames, err = NewFrames(p.manager, p); err != nil {
		return nil, err
	}
	if p.obligations, err = NewProofObligationStore(p, true); err != nil {
		return nil, err
	}
	committed = true
	return p, nil
}

func (p *PredicateTransformer) SetDerivationPolicy(retain, reverseChildren bool) error {
	if err := p.open(); err != nil {
		return err
	}
	p.useDerivations = retain
	p.reverseChildren = reverseChildren
	return nil
}

// SetReachFactPolicy selects the source auxiliary-elimination and implicant
// policies used by subsequent concrete reach-fact construction.
func (p *PredicateTransformer) SetReachFactPolicy(eliminateAuxiliaries, reachDNF bool) error {
	if err := p.open(); err != nil {
		return err
	}
	p.eliminateAuxiliaries = eliminateAuxiliaries
	p.reachDNF = reachDNF
	return nil
}

// SetGroundCTI selects whether derivation projection must eliminate every
// variable by falling back to its model value.
func (p *PredicateTransformer) SetGroundCTI(value bool) error {
	if err := p.open(); err != nil {
		return err
	}
	p.groundCTI = value
	return nil
}

// SetWeakAbstraction selects the native obligation-weakening solver policy.
func (p *PredicateTransformer) SetWeakAbstraction(value bool) error {
	if err := p.open(); err != nil {
		return err
	}
	p.weakAbstraction = value
	return nil
}

// NewPredicateTransformer creates the live solver/frame bridge after rule
// conversion has produced the initial-state and transition formulas.
func NewPredicateTransformer(owner *Manager, name string, initial, transition ast.ASTHandle, config PropSolverConfig) (_ *PredicateTransformer, resultErr error) {
	p, err := newPredicateTransformer(owner, name, config)
	if err != nil {
		return nil, err
	}
	if err = p.initializeExtendLiteral(name); err != nil {
		return nil, errors.Join(err, p.Close())
	}
	if err = p.InitializeSystem(initial, transition); err != nil {
		return nil, errors.Join(err, p.Close())
	}
	return p, nil
}

func NewRulePredicateTransformer(owner *Manager, head ast.ASTHandle, config PropSolverConfig) (_ *PredicateTransformer, resultErr error) {
	if owner == nil || owner.open() != nil {
		return nil, ErrManagerOwner
	}
	kind, err := owner.manager.GetKind(head)
	if err != nil || kind != ast.ASTFuncDecl {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return nil, err
	}
	symbol, err := owner.manager.GetASTName(head)
	if err != nil {
		return nil, err
	}
	name, err := owner.manager.Symbols.String(symbol)
	if err != nil {
		return nil, err
	}
	p, err := newPredicateTransformer(owner, name, config)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, p.Close())
		}
	}()
	if p.head, err = p.manager.NewRef(head); err != nil {
		return nil, err
	}
	if err = p.initializeSignature(name, head); err != nil {
		return nil, err
	}
	if err = p.initializeExtendLiteral(name); err != nil {
		return nil, err
	}
	committed = true
	return p, nil
}

func (p *PredicateTransformer) initializeSignature(name string, head ast.ASTHandle) error {
	arity, err := p.manager.GetArity(head)
	if err != nil {
		return err
	}
	for i := uint32(0); i < arity; i++ {
		sort, err := p.manager.GetDomain(head, i)
		if err != nil {
			return err
		}
		symbol, err := p.manager.Symbols.Intern(fmt.Sprintf("%s_%d", name, i))
		if err != nil {
			return err
		}
		state, err := p.manager.MkFuncDecl(symbol, nil, sort, ast.ASTFuncDeclInfo{})
		if err != nil {
			return err
		}
		old, err := p.owner.OldPredicate(state, 0)
		if err != nil {
			return err
		}
		ref, err := p.manager.NewRef(old)
		if err != nil {
			return err
		}
		p.signature = append(p.signature, ref)
	}
	return nil
}

func (p *PredicateTransformer) initializeExtendLiteral(name string) error {
	boolean, err := p.manager.MkBoolSort()
	if err != nil {
		return err
	}
	symbol, err := p.manager.Symbols.Intern(name + "_ext0")
	if err != nil {
		return err
	}
	application, err := p.manager.MkConst(symbol, boolean)
	if err != nil {
		return err
	}
	declaration, err := p.manager.GetDecl(application)
	if err != nil {
		return err
	}
	current, err := p.owner.NewPredicate(declaration)
	if err != nil {
		return err
	}
	currentApplication, err := p.manager.MkApp(current, nil)
	if err != nil {
		return err
	}
	negated, err := p.manager.MkNot(currentApplication)
	if err != nil {
		return err
	}
	p.extendLiteral, err = p.manager.NewRef(negated)
	return err
}

func (p *PredicateTransformer) InitializeSystem(initial, transition ast.ASTHandle) (resultErr error) {
	if err := p.open(); err != nil {
		return err
	}
	if p.initialized {
		return ast.ErrASTDomain
	}
	initialRef, err := p.manager.NewRef(initial)
	if err != nil {
		return err
	}
	transitionRef, err := p.manager.NewRef(transition)
	if err != nil {
		return errors.Join(err, p.manager.DelRef(&initialRef))
	}
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&transitionRef), p.manager.DelRef(&initialRef))
		}
	}()
	if err = p.solver.Assert(transition); err != nil {
		return err
	}
	if err = p.solver.AssertAtLevel(initial, 0); err != nil {
		return err
	}
	if err = p.frames.AddFrame(); err != nil {
		return err
	}
	p.initial = initialRef
	p.transition = transitionRef
	p.initialized = true
	committed = true
	return nil
}

func (p *PredicateTransformer) Head() (ast.ASTHandle, error) {
	if err := p.open(); err != nil || p.head == (ast.ASTRef{}) {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, err
	}
	return p.manager.RefGet(p.head)
}

func (p *PredicateTransformer) Signature() ([]ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, len(p.signature))
	for i := range p.signature {
		handle, err := p.manager.RefGet(p.signature[i])
		if err != nil {
			return nil, err
		}
		result[i] = handle
	}
	return result, nil
}

func (p *PredicateTransformer) open() error {
	if p == nil || p.closed || p.owner == nil || p.manager == nil || p.solver == nil || p.reachSolver == nil || p.frames == nil {
		return ErrManagerOwner
	}
	return nil
}

func (p *PredicateTransformer) EnsureLevel(level uint32) error {
	if err := p.open(); err != nil {
		return err
	}
	if level == InfinityLevel {
		return nil
	}
	size, err := p.frames.Size()
	if err != nil {
		return err
	}
	for size <= level {
		if err = p.frames.AddFrame(); err != nil {
			return err
		}
		if err = p.solver.EnsureLevel(size); err != nil {
			return err
		}
		size++
	}
	return nil
}

func (p *PredicateTransformer) AddLemmaCore(lemma *Lemma) error {
	if err := p.open(); err != nil {
		return err
	}
	if lemma == nil || lemma.manager != p.manager {
		return ErrManagerOwner
	}
	body, err := lemma.Body()
	if err != nil {
		return err
	}
	ground, err := p.manager.IsGround(body)
	if err != nil {
		return err
	}
	level, err := lemma.Level()
	if err != nil {
		return err
	}
	if level == InfinityLevel {
		p.invariants++
	}
	if ground {
		if level == InfinityLevel {
			err = p.solver.Assert(body)
		} else {
			if err = p.EnsureLevel(level); err == nil {
				err = p.solver.AssertAtLevel(body, level)
			}
		}
		if err != nil {
			return err
		}
	}
	next := level
	if next != InfinityLevel {
		next++
	}
	for _, user := range p.uses {
		if err = user.addLemmaFromChild(p, lemma, next); err != nil {
			return err
		}
	}
	return nil
}

func (p *PredicateTransformer) IsInvariant(level uint32, formula ast.ASTHandle) (bool, uint32, error) {
	if err := p.open(); err != nil {
		return false, 0, err
	}
	extension, err := p.manager.RefGet(p.extendLiteral)
	if err != nil {
		return false, 0, err
	}
	negated, err := p.manager.MkNotSimplified(formula)
	if err != nil {
		return false, 0, err
	}
	result, err := p.solver.CheckAssumptions([]ast.ASTHandle{negated}, nil, PropCheckOptions{
		SubsetBasedCore: true,
		Level:           &level,
		Background:      []ast.ASTHandle{extension},
	})
	if err != nil {
		return false, 0, err
	}
	defer func() { _ = result.Close(p.manager) }()
	if result.Status != lbool.False {
		return false, 0, nil
	}
	return true, result.UsesLevel, nil
}

func (p *PredicateTransformer) IsBlocked(obligation *ProofObligation) (bool, uint32, error) {
	if err := p.open(); err != nil {
		return false, 0, err
	}
	if obligation == nil || obligation.valid() != nil || obligation.transformer != p {
		return false, 0, ErrManagerOwner
	}
	level, err := obligation.Level()
	if err != nil {
		return false, 0, err
	}
	if err = p.EnsureLevel(level); err != nil {
		return false, 0, err
	}
	post, err := obligation.Post()
	if err != nil {
		return false, 0, err
	}
	result, err := p.solver.CheckAssumptions([]ast.ASTHandle{post}, nil, PropCheckOptions{Level: &level})
	if err != nil {
		return false, 0, err
	}
	defer func() { _ = result.Close(p.manager) }()
	if result.Status != lbool.False {
		return false, 0, nil
	}
	return true, result.UsesLevel, nil
}

func (p *PredicateTransformer) IsQuantifierBlocked(obligation *ProofObligation) (_ bool, resultErr error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if obligation == nil || obligation.valid() != nil || obligation.transformer != p {
		return false, ErrManagerOwner
	}
	level, err := obligation.Level()
	if err != nil {
		return false, err
	}
	lemmas, err := p.frames.FrameGEQLemmas(level)
	if err != nil {
		return false, err
	}
	post, err := obligation.Post()
	if err != nil {
		return false, err
	}
	context, err := p.owner.Fresh(2)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, context.Close()) }()
	for _, lemma := range lemmas {
		if err = context.Assert(lemma); err != nil {
			return false, err
		}
	}
	if err = context.Assert(post); err != nil {
		return false, err
	}
	status, err := context.CheckSat(nil)
	return status == lbool.False, err
}

// AddLemma transfers ownership of the new lemma to the transformer's frames.
func (p *PredicateTransformer) AddLemma(formula ast.ASTHandle, level uint32) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	lemma, err := NewLemma(p.owner, formula, level)
	if err != nil {
		return false, err
	}
	added, err := p.frames.AddLemma(lemma)
	if err != nil {
		_ = lemma.Close()
	}
	return added, err
}

func (p *PredicateTransformer) PropagateToNextLevel(level uint32) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	p.propagations++
	return p.frames.PropagateToNextLevel(level)
}

func (p *PredicateTransformer) PropagateToInfinity(level uint32) error {
	if err := p.open(); err != nil {
		return err
	}
	return p.frames.PropagateToInfinity(level)
}

func (p *PredicateTransformer) NumLevels() (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	return p.frames.Size()
}

func (p *PredicateTransformer) Formulas(level uint32, addAxioms bool) (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	var formulas []ast.ASTHandle
	if addAxioms {
		if !p.initialized {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		background, err := p.owner.Background()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		formulas = append(formulas, background)
		var state ast.ASTHandle
		if level == 0 {
			state, err = p.manager.RefGet(p.initial)
		} else {
			state, err = p.manager.RefGet(p.transition)
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
		formulas = append(formulas, state)
	}
	lemmas, err := p.frames.FrameGEQLemmas(level)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	formulas = append(formulas, lemmas...)
	return p.manager.MkAndArray(formulas)
}

func (p *PredicateTransformer) CollectStatistics(destination *statistics.Statistics) error {
	if err := p.open(); err != nil {
		return err
	}
	if destination == nil {
		return ast.ErrASTHandle
	}
	if err := p.solver.CollectStatistics(destination); err != nil {
		return err
	}
	lemmas, err := p.frames.LemmaSize()
	if err != nil {
		return err
	}
	destination.UpdateUint64("SPACER num propagations", p.propagations)
	destination.UpdateUint("SPACER num properties", uint32(lemmas))
	destination.UpdateUint64("SPACER num invariants", p.invariants)
	destination.UpdateUint64("bool inductive gen", p.boolGeneralizations)
	destination.UpdateUint64("bool inductive gen failures", p.boolGeneralizerFailures)
	destination.UpdateDouble("time.spacer.solve.reach.gen.bool_ind", p.boolGeneralizerTime.Seconds())
	return nil
}

func (p *PredicateTransformer) ResetStatistics() error {
	if err := p.open(); err != nil {
		return err
	}
	if err := p.solver.ResetStatistics(); err != nil {
		return err
	}
	p.propagations = 0
	p.invariants = 0
	p.boolGeneralizations = 0
	p.boolGeneralizerFailures = 0
	p.boolGeneralizerTime = 0
	return nil
}

func (p *PredicateTransformer) Close() (result error) {
	if p == nil {
		return ErrManagerOwner
	}
	if p.closed {
		return nil
	}
	if p.frames != nil {
		if p.obligations != nil {
			result = errors.Join(result, p.obligations.Close())
		}
		result = errors.Join(result, p.frames.Close())
	}
	if p.solver != nil {
		result = errors.Join(result, p.solver.Close())
	}
	if p.reachSolver != nil {
		result = errors.Join(result, p.reachSolver.Close())
	}
	if p.manager != nil {
		for i := len(p.reachFacts) - 1; i >= 0; i-- {
			result = errors.Join(result, p.reachFacts[i].Close())
		}
		result = errors.Join(result, releaseNormalizeRefs(p.manager, p.reachCaseVariables))
		for i := len(p.rules) - 1; i >= 0; i-- {
			result = errors.Join(result, p.rules[i].rule.Close())
		}
		for i := len(p.ruleReferences) - 1; i >= 0; i-- {
			result = errors.Join(result, p.manager.DelRef(&p.ruleReferences[i]))
		}
		if p.extendLiteral != (ast.ASTRef{}) {
			result = errors.Join(result, p.manager.DelRef(&p.extendLiteral))
		}
		for i := len(p.signature) - 1; i >= 0; i-- {
			result = errors.Join(result, p.manager.DelRef(&p.signature[i]))
		}
		if p.head != (ast.ASTRef{}) {
			result = errors.Join(result, p.manager.DelRef(&p.head))
		}
		if p.transition != (ast.ASTRef{}) {
			result = errors.Join(result, p.manager.DelRef(&p.transition))
		}
		if p.initial != (ast.ASTRef{}) {
			result = errors.Join(result, p.manager.DelRef(&p.initial))
		}
	}
	if result == nil {
		p.frames = nil
		p.obligations = nil
		p.solver = nil
		p.reachSolver = nil
		p.signature = nil
		p.rules = nil
		p.ruleReferences = nil
		p.uses = nil
		p.reachFacts = nil
		p.reachCaseVariables = nil
		p.owner = nil
		p.manager = nil
		p.closed = true
	}
	return result
}
