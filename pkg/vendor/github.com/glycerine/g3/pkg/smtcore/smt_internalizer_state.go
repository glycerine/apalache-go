package smtcore

// Source: src/smt/smt_internalizer.cpp Boolean-variable, enode, and literal
// registration methods.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uheap "github.com/glycerine/g3/pkg/util/heap"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	z3rand "github.com/glycerine/g3/pkg/util/random"
)

type smtInternalizerScope struct {
	goalQueue            smtRelevanceGoalScope
	relActLimit          int
	relActHead           int
	relevanceFIFO        smtRelevanceFIFOScope
	trueFirstLimit       int
	theoryCaseSplitLimit int
	booleanLimit         int
	enodeLimit           int
	pendingLimit         int
	pendingHead          int
	equalityLimit        int
	assignmentLimit      int
	clauseLimit          int
	justificationLimit   int
	theoryLimit          int
	assignedLimit        int
	propagationHead      int
	theoryEqLimit        int
	theoryEqHead         int
	theoryDiseqLimit     int
	theoryDiseqHead      int
	atomHead             int
	notifyLimit          int
	enodeFlagLimit       int
	atomNotifiedLimit    int
	simplifyHead         int
	inconsistent         bool
	conflict             SMTBooleanJustification
	conflictLiteral      SMTLiteral
	disequalities        map[smtEnodePairKey]struct{}
	disequalityReasons   map[smtEnodePairKey]SMTLiteral
}

type smtInternalizerAssignmentTrail struct {
	variable      int32
	value         lbool.LBool
	justification SMTBooleanJustification
	scopeLevel    uint32
}

type smtInternalizerTheoryTrail struct {
	node      *SMTEnode
	root      *SMTEnode
	theory    SMTTheoryID
	old       SMTTheoryVar
	rootAdded bool
}

type smtInternalizerNotifyTrail struct {
	variable int32
	theory   SMTTheoryID
}

type smtEqualityNodeSnapshot struct {
	node            *SMTEnode
	root, next      *SMTEnode
	classSize       uint32
	theoryVariables SMTTheoryVarList
	trans           SMTTransJustification
}

type smtEqualitySnapshot struct {
	nodes []smtEqualityNodeSnapshot
}

type SMTTheoryPropagation struct {
	Theory      SMTTheoryID
	Left, Right SMTTheoryVar
}

type SMTInternalizedClause struct {
	Literals []SMTLiteral
	Root     bool
	Binary   bool
	Proof    ast.ASTHandle
	clause   *SMTClause
	owner    SMTJustification
}

type SMTInternalizerCongruence struct {
	Node     *SMTEnode
	Existing *SMTEnode
	Commuted bool
}

// Source context::new_eq: explicit theory equalities and congruences share
// one FIFO and retain their concrete tagged equality justification.
type smtPendingEquality struct {
	left, right   *SMTEnode
	justification SMTEqualityJustification
}

type SMTInternalizerState struct {
	queueTrace                 *diagnostic.TraceTags // Borrowed control; nil uses process trace tags.
	delayedActivity            *uheap.Int
	activityCacheRetiring      bool
	cachedActivity             bool
	activityCacheDomain        []ast.ASTHandle
	activityCacheDrop          int
	basicActivity              *uheap.Int
	relevanceGoal              smtRelevanceGoalQueue
	relevanceActivity          smtRelevanceActivityQueue
	relevanceFIFO              smtRelevanceFIFO
	lemmaProblem               *smtLemmaProblem // Interrupted lexical roots, retired before the manager.
	theoryProgress             uint64
	manager                    *ast.ASTManager
	theoryRewriter             *rewriter.TheoryRewriter // borrowed from the kernel
	smtParams                  *SMTParams               // borrowed context::m_fparams
	contextParams              *uparams.ParamsRef       // borrowed context::m_params
	context                    *SMTCheckerContext
	congruence                 *SMTCongruenceTable
	quantifiers                *SMTQuantifierRegistry
	quantifierManager          *SMTQuantifierManager
	assertedHasQuantifiers     *bool                 // borrowed context::has_quantifiers state
	assertedInconsistent       *bool                 // borrowed asserted-formula inconsistency state
	assertedFormulas           *[]SMTKernelAssertion // borrowed context asserted-formula owner
	relevancy                  *SMTRelevancyPropagator
	theories                   map[ast.FamilyID]SMTTheoryInternalizer
	theoryOrder                []SMTTheoryInternalizer
	boolVar                    map[ast.ASTHandle]int32
	boolExpr                   []ast.ASTHandle
	booleanData                []SMTBoolVarData
	booleanCreation            []uint64
	nextBooleanCreation        uint64
	activity                   []float64
	activityIncrement          float64
	activityInverseDecay       float64
	random                     z3rand.Generator
	randomInitialActivity      uint32
	caseSplitStrategy          uint32
	theoryAwareBranching       *smtTheoryAwareBranchingQueue
	theoryCaseSplit            bool
	theoryCaseSplitSets        [][]SMTLiteral
	allTheoryCaseSplitLiterals map[SMTLiteral]struct{}
	literalTheoryCaseSplitSets map[SMTLiteral][][]SMTLiteral
	activityCache              map[ast.ASTHandle]float64
	assignment                 []lbool.LBool
	watches                    []SMTWatchList
	booleanStack               []ast.ASTHandle
	enodeStack                 []*SMTEnode
	scopes                     []smtInternalizerScope
	cachedGeneration           map[ast.ASTHandle]uint32
	pending                    []smtPendingEquality
	pendingHead                int
	equalityTrail              []smtEqualitySnapshot
	disequalityReasons         map[smtEnodePairKey]SMTLiteral
	dynamicAck                 *smtDynAckManager
	assignmentTrail            []smtInternalizerAssignmentTrail
	theoryTrail                []smtInternalizerTheoryTrail
	theoryEqualities           []SMTTheoryPropagation
	theoryDisequalities        []SMTTheoryPropagation
	theoryEqualityHead         int
	theoryDisequalityHead      int
	atomPropagationHead        int
	notifyTrail                []smtInternalizerNotifyTrail
	enodeFlagTrail             []int32
	trueFirstTrail             []int32
	atomNotified               map[int32]bool
	atomNotifiedTrail          []int32
	relevancyLevel             uint32
	newCore2thEq               bool
	clauses                    []SMTInternalizedClause
	ownedJustifications        []SMTJustification
	assignedLiterals           []SMTLiteral
	propagationHead            int
	conflict                   SMTBooleanJustification
	conflictLiteral            SMTLiteral
	inconsistent               bool
	generation                 uint32
	scopeLevel                 uint32
	statistics                 SMTContextStatistics
	finalCheckIndex            int
	incompleteTheories         []SMTTheoryInternalizer
	theoriesSetup              bool
	binaryClauseOptimization   bool
	clauseMaintenance          SMTClauseMaintenanceConfig
	conflictsSinceLemmaGC      uint32
	lemmaGCThreshold           uint32
	simplifyHead               int
	simplifyCounter            int
	learnedUnits               []smtPersistentLearnedUnit
	searchBaseLevel            uint32
	searchUserBaseLevel        uint32
	searchAgility              float64
	searchAgilityFactor        float64
	searchRestartAdaptive      bool
	searching                  bool
	closed                     bool
}

func NewSMTInternalizerState(manager *ast.ASTManager, context *SMTCheckerContext, quantifiers *SMTQuantifierRegistry) (*SMTInternalizerState, error) {
	if manager == nil || context == nil || context.manager != manager || quantifiers != nil && quantifiers.manager != manager {
		return nil, ast.ErrASTHandle
	}
	table, err := NewSMTCongruenceTable(manager)
	if err != nil {
		return nil, err
	}
	state := &SMTInternalizerState{
		manager: manager, context: context, congruence: table, quantifiers: quantifiers,
		boolVar: make(map[ast.ASTHandle]int32), boolExpr: make([]ast.ASTHandle, 1), booleanData: make([]SMTBoolVarData, 1), booleanCreation: make([]uint64, 1),
		activity: make([]float64, 1), assignment: []lbool.LBool{lbool.True, lbool.False}, watches: make([]SMTWatchList, 2),
		cachedGeneration: make(map[ast.ASTHandle]uint32), theories: make(map[ast.FamilyID]SMTTheoryInternalizer),
		disequalityReasons:    make(map[smtEnodePairKey]SMTLiteral),
		conflictLiteral:       SMTNullLiteral,
		activityIncrement:     1,
		activityInverseDecay:  1.052,
		randomInitialActivity: 1,
		caseSplitStrategy:     1,
		relevanceFIFO:         smtRelevanceFIFO{baseBooleanLimit: -1},
		activityCache:         make(map[ast.ASTHandle]float64),
		clauseMaintenance:     defaultSMTClauseMaintenanceConfig(),
		lemmaGCThreshold:      5000,
		newCore2thEq:          true,
	}
	state.booleanData[0].Initialize(0)
	dynamicAck, err := newSMTDynAckManager(state, defaultSMTDynAckState())
	if err != nil {
		_ = table.Close()
		return nil, err
	}
	state.dynamicAck = dynamicAck
	return state, nil
}

// ConfigureDynamicAck installs source dynamic-Ackermann parameters before any
// expressions or scopes are created.
func (s *SMTInternalizerState) ConfigureDynamicAck(config SMTDynAckState) error {
	if err := s.open(); err != nil {
		return err
	}
	if len(s.booleanStack) != 0 || len(s.enodeStack) != 0 || len(s.scopes) != 0 {
		return ast.ErrASTDomain
	}
	replacement, err := newSMTDynAckManager(s, config)
	if err != nil {
		return err
	}
	if err = s.dynamicAck.close(); err != nil {
		_ = replacement.close()
		return err
	}
	s.dynamicAck = replacement
	return nil
}

// UpdateDynamicAck changes the source scalar parameters without discarding
// occurrence counts, pending candidates, or installed dynamic-Ack clauses.
func (s *SMTInternalizerState) UpdateDynamicAck(config SMTDynAckState) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.dynamicAck.updateConfig(config)
}

func (s *SMTInternalizerState) RegisterTheory(theory SMTTheoryInternalizer) error {
	if err := s.open(); err != nil {
		return err
	}
	if theory == nil || theory.FamilyID() == ast.NullFamilyID || theory.FamilyID() == s.manager.BasicFamilyID() || s.theories[theory.FamilyID()] != nil || len(s.scopes) != 0 || s.theoriesSetup {
		return ast.ErrASTDomain
	}
	family := theory.FamilyID()
	s.theories[family] = theory
	s.theoryOrder = append(s.theoryOrder, theory)
	if err := theory.Initialize(s); err != nil {
		delete(s.theories, family)
		s.theoryOrder = s.theoryOrder[:len(s.theoryOrder)-1]
		return err
	}
	return nil
}

// Source context::is_relevant includes all internalized expressions when
// filtering is disabled. A standalone read-side context without a relevancy
// service retains its explicitly marked view.
func (s *SMTInternalizerState) isRelevant(expression ast.ASTHandle) bool {
	return s.context.isRelevant(expression)
}

func (s *SMTInternalizerState) Theory(family ast.FamilyID) SMTTheoryInternalizer {
	if s == nil {
		return nil
	}
	return s.theories[family]
}

// SetBinaryClauseOptimization selects the source compact representation for
// eligible two-literal clauses. It must be configured before internalization.
func (s *SMTInternalizerState) SetBinaryClauseOptimization(enabled bool) error {
	if err := s.open(); err != nil {
		return err
	}
	if len(s.booleanStack) != 0 || len(s.enodeStack) != 0 || len(s.clauses) != 0 || len(s.scopes) != 0 {
		return ast.ErrASTDomain
	}
	if enabled {
		proofs, err := s.manager.ProofsEnabled()
		if err != nil {
			return err
		}
		if proofs {
			return ast.ErrASTDomain
		}
	}
	s.binaryClauseOptimization = enabled
	return nil
}

func (s *SMTInternalizerState) SetVariableTheory(variable int32, theory SMTTheoryID) error {
	if err := s.open(); err != nil {
		return err
	}
	if variable < SMTFirstBoolVar || int(variable) >= len(s.booleanData) || theory == SMTNullTheoryID || s.theories[theory] == nil || s.booleanData[variable].IsTheoryAtom() {
		return ast.ErrASTDomain
	}
	if s.scopeLevel > s.booleanData[variable].InternalLevel() {
		s.notifyTrail = append(s.notifyTrail, smtInternalizerNotifyTrail{variable: variable, theory: s.booleanData[variable].Theory()})
	}
	return s.booleanData[variable].SetNotifyTheory(theory)
}

func (s *SMTInternalizerState) attachTheoryVariable(node *SMTEnode, theory SMTTheoryID, variable SMTTheoryVar) error {
	if err := s.open(); err != nil {
		return err
	}
	plugin := s.theories[theory]
	if node == nil || node.manager != s.manager || theory == SMTNullTheoryID || variable == SMTNullTheoryVar || plugin == nil {
		return ast.ErrASTDomain
	}
	old := node.TheoryVar(theory)
	trail := smtInternalizerTheoryTrail{node: node, theory: theory, old: old}
	if old == SMTNullTheoryVar {
		root := node.root
		if root == nil {
			return ast.ErrASTDomain
		}
		rootVariable := root.TheoryVar(theory)
		if err := node.AddTheoryVar(theory, variable); err != nil {
			return err
		}
		if root != node && rootVariable == SMTNullTheoryVar {
			if err := root.AddTheoryVar(theory, variable); err != nil {
				_ = node.DeleteTheoryVar(theory)
				return err
			}
			trail.root, trail.rootAdded = root, true
		}
		if rootVariable == SMTNullTheoryVar {
			if err := s.enqueueExistingDisequalities(root, variable, plugin); err != nil {
				if trail.rootAdded {
					_ = root.DeleteTheoryVar(theory)
				}
				_ = node.DeleteTheoryVar(theory)
				return err
			}
		} else if root != node {
			if err := s.enqueueTheoryEquality(theory, rootVariable, variable); err != nil {
				_ = node.DeleteTheoryVar(theory)
				return err
			}
		}
	} else if err := node.ReplaceTheoryVar(theory, variable); err != nil {
		return err
	} else if err := s.enqueueTheoryEquality(theory, variable, old); err != nil {
		_ = node.ReplaceTheoryVar(theory, old)
		return err
	}
	s.theoryTrail = append(s.theoryTrail, trail)
	return nil
}

func (s *SMTInternalizerState) open() error {
	if s == nil || s.manager == nil || s.context == nil || s.congruence == nil || s.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (s *SMTInternalizerState) SetGeneration(generation uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	s.generation = generation
	if generation > s.statistics.MaxGeneration {
		s.statistics.MaxGeneration = generation
	}
	return nil
}

func (s *SMTInternalizerState) SetCachedGeneration(expression ast.ASTHandle, generation uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	if _, err := s.manager.GetKind(expression); err != nil {
		return err
	}
	s.cachedGeneration[expression] = generation
	return nil
}

func (s *SMTInternalizerState) BoolVar(expression ast.ASTHandle) (int32, bool) {
	if s == nil {
		return SMTNullBoolVar, false
	}
	variable, found := s.boolVar[expression]
	return variable, found
}

func (s *SMTInternalizerState) BoolExpression(variable int32) (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if variable < SMTFirstBoolVar || int(variable) >= len(s.boolExpr) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return s.boolExpr[variable], nil
}

func (s *SMTInternalizerState) BooleanData(variable int32) (*SMTBoolVarData, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if variable < SMTFirstBoolVar || int(variable) >= len(s.booleanData) {
		return nil, ast.ErrASTDomain
	}
	return &s.booleanData[variable], nil
}

func (s *SMTInternalizerState) MkBoolVar(expression ast.ASTHandle) (int32, error) {
	if err := s.open(); err != nil {
		return SMTNullBoolVar, err
	}
	if _, found := s.boolVar[expression]; found {
		return SMTNullBoolVar, ast.ErrASTDomain
	}
	sort, err := s.manager.GetSort(expression)
	if err != nil {
		return SMTNullBoolVar, err
	}
	boolean, err := s.manager.MkBoolSort()
	if err != nil {
		return SMTNullBoolVar, err
	}
	if sort != boolean || s.manager.IsTrue(expression) || s.manager.IsFalse(expression) {
		return SMTNullBoolVar, ast.ErrASTDomain
	}
	if err = s.manager.IncRef(expression); err != nil {
		return SMTNullBoolVar, err
	}
	variable := int32(len(s.boolExpr))
	s.boolVar[expression] = variable
	s.boolExpr = append(s.boolExpr, expression)
	data := SMTBoolVarData{}
	data.Initialize(s.scopeLevel)
	s.booleanData = append(s.booleanData, data)
	s.nextBooleanCreation++
	s.booleanCreation = append(s.booleanCreation, s.nextBooleanCreation)
	initialActivity := 0.0
	if s.randomInitialActivity == 2 || s.randomInitialActivity == 1 && s.searching {
		initialActivity = -float64(s.random.Next()%1000) / 1000
	}
	if s.cachedActivity || s.caseSplitStrategy == 2 {
		if cached, found := s.activityCache[expression]; found {
			initialActivity = cached
		}
	}
	s.activity = append(s.activity, initialActivity)
	s.assignment = append(s.assignment, lbool.Undef, lbool.Undef)
	s.watches = append(s.watches, SMTWatchList{}, SMTWatchList{})
	s.booleanStack = append(s.booleanStack, expression)
	s.statistics.NumMkBoolVar++
	if err := s.basicActivityCreated(variable); err != nil {
		return SMTNullBoolVar, err
	}
	if err := s.relevanceActivityCreated(variable); err != nil {
		return SMTNullBoolVar, err
	}
	if s.theoryAwareBranching != nil {
		if err := s.theoryAwareBranching.created(variable); err != nil {
			return SMTNullBoolVar, err
		}
	}
	return variable, nil
}

func (s *SMTInternalizerState) Literal(expression ast.ASTHandle) (SMTLiteral, error) {
	if err := s.open(); err != nil {
		return SMTNullLiteral, err
	}
	if s.manager.IsTrue(expression) {
		return SMTTrueLiteral, nil
	}
	if s.manager.IsFalse(expression) {
		return SMTFalseLiteral, nil
	}
	kind, err := s.manager.GetKind(expression)
	if err != nil {
		return SMTNullLiteral, err
	}
	if kind == ast.ASTApp {
		family, err := s.manager.GetFamilyID(expression)
		if err != nil {
			return SMTNullLiteral, err
		}
		op, err := s.manager.GetDeclKind(expression)
		if err != nil {
			return SMTNullLiteral, err
		}
		if family == s.manager.BasicFamilyID() && op == ast.ASTOpNot {
			child, err := s.manager.GetArg(expression, 0)
			if err != nil {
				return SMTNullLiteral, err
			}
			// Reserved truth literals have no ordinary boolVar table entry.
			if s.manager.IsTrue(child) {
				return SMTFalseLiteral, nil
			}
			if s.manager.IsFalse(child) {
				return SMTTrueLiteral, nil
			}
			variable, found := s.boolVar[child]
			if !found {
				return SMTNullLiteral, ast.ErrASTDomain
			}
			return NewSMTLiteral(variable, true), nil
		}
	}
	variable, found := s.boolVar[expression]
	if !found {
		return SMTNullLiteral, ast.ErrASTDomain
	}
	return NewSMTLiteral(variable, false), nil
}

func (s *SMTInternalizerState) SetAssignment(variable int32, value lbool.LBool) error {
	if err := s.open(); err != nil {
		return err
	}
	if variable < SMTFirstBoolVar || int(variable) >= len(s.boolExpr) || value < lbool.False || value > lbool.True {
		return ast.ErrASTDomain
	}
	positive := NewSMTLiteral(variable, false)
	old := s.assignment[positive.Index()]
	if old == value {
		return nil
	}
	if len(s.scopes) != 0 {
		s.assignmentTrail = append(s.assignmentTrail, smtInternalizerAssignmentTrail{
			variable: variable, value: old, justification: s.booleanData[variable].Justification(), scopeLevel: s.booleanData[variable].ScopeLevel,
		})
	}
	s.assignment[positive.Index()] = value
	s.assignment[positive.Negated().Index()] = value.Negated()
	if value == lbool.Undef {
		if err := s.basicActivityUnassigned(variable); err != nil {
			return err
		}
	}
	if value == lbool.Undef && s.theoryAwareBranching != nil {
		if err := s.theoryAwareBranching.unassigned(variable); err != nil {
			return err
		}
	}
	return s.context.SetAssignment(s.boolExpr[variable], value)
}

func (s *SMTInternalizerState) Assignment(literal SMTLiteral) (lbool.LBool, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, err
	}
	if literal.Index() < 0 || int(literal.Index()) >= len(s.assignment) {
		return lbool.Undef, ast.ErrASTDomain
	}
	return s.assignment[literal.Index()], nil
}

func (s *SMTInternalizerState) WatchList(literal SMTLiteral) (*SMTWatchList, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if literal.Index() < 0 || int(literal.Index()) >= len(s.watches) {
		return nil, ast.ErrASTDomain
	}
	return &s.watches[literal.Index()], nil
}

func (s *SMTInternalizerState) MkEnode(expression ast.ASTHandle, suppressArguments, mergeTruthValue, congruenceEnabled bool) (*SMTEnode, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if s.context.enodes[expression] != nil {
		return nil, ast.ErrASTDomain
	}
	generation := s.generation
	if cached, found := s.cachedGeneration[expression]; found {
		generation = cached
	}
	if err := s.manager.IncRef(expression); err != nil {
		return nil, err
	}
	node, err := NewSMTEnode(s.manager, expression, s.context.enodes, SMTEnodeOptions{
		Generation: generation, InternalScopeLevel: s.scopeLevel, SuppressArguments: suppressArguments,
		MergeTruthValue: mergeTruthValue, CongruenceEnabled: congruenceEnabled, UpdateChildrenParent: true,
	})
	if err != nil {
		_ = s.manager.DecRef(expression)
		return nil, err
	}
	count := node.NumArgs()
	if count == 0 {
		unique, err := s.manager.IsUniqueValue(expression)
		if err != nil {
			_ = node.Delete(true)
			_ = s.manager.DecRef(expression)
			return nil, err
		}
		if unique {
			if err = node.MarkAsInterpreted(); err != nil {
				_ = node.Delete(true)
				_ = s.manager.DecRef(expression)
				return nil, err
			}
		}
	} else if node.IsTrueEquality() {
		node.congruence = node
	} else if congruenceEnabled {
		inserted, err := s.congruence.Insert(node)
		if err != nil {
			_ = node.Delete(true)
			_ = s.manager.DecRef(expression)
			return nil, err
		}
		node.congruence = inserted.Node
		if inserted.Node != node {
			s.pending = append(s.pending, smtPendingEquality{node, inserted.Node, NewSMTEqualityCongruence(inserted.Commuted)})
		}
	} else {
		node.congruence = node
	}
	// Source mk_enode does not reset an expression's existing relevance.
	// Gate-context Boolean atoms can be marked before a term-context enode
	// is needed, including during a later search scope.
	if err = s.context.RegisterEnode(node, s.context.relevant[expression]); err != nil {
		if node.congruence == node && count != 0 && congruenceEnabled && !node.IsTrueEquality() {
			_, _ = s.congruence.Erase(node)
		}
		_ = node.Delete(true)
		_ = s.manager.DecRef(expression)
		return nil, err
	}
	s.enodeStack = append(s.enodeStack, node)
	s.statistics.NumMkEnode++
	// Source mk_enode assigns a newly created true equality immediately.
	// Its operands may already have merged before this atom was internalized.
	if node.IsTrueEquality() {
		if err := s.propagateTrueEquality(node); err != nil {
			return nil, err
		}
	}
	return node, nil
}

func (s *SMTInternalizerState) PendingCongruences() []SMTInternalizerCongruence {
	if s == nil {
		return nil
	}
	var result []SMTInternalizerCongruence
	for _, entry := range s.pending {
		if commuted, congruence := entry.justification.UsedCommutativity(); congruence {
			result = append(result, SMTInternalizerCongruence{entry.left, entry.right, commuted})
		}
	}
	return result
}

func (s *SMTInternalizerState) PushScope() error {
	if err := s.open(); err != nil {
		return err
	}
	disequalities := make(map[smtEnodePairKey]struct{}, len(s.context.disequalities))
	for key := range s.context.disequalities {
		disequalities[key] = struct{}{}
	}
	reasons := make(map[smtEnodePairKey]SMTLiteral, len(s.disequalityReasons))
	for key, literal := range s.disequalityReasons {
		reasons[key] = literal
	}
	s.scopes = append(s.scopes, smtInternalizerScope{
		goalQueue:     s.relevanceGoal.snapshot(s.scopeLevel),
		relevanceFIFO: s.relevanceFIFO.snapshot(),
		relActLimit:   len(s.relevanceActivity.entries), relActHead: s.relevanceActivity.head,
		trueFirstLimit:       len(s.trueFirstTrail),
		theoryCaseSplitLimit: len(s.theoryCaseSplitSets),
		booleanLimit:         len(s.booleanStack), enodeLimit: len(s.enodeStack), pendingLimit: len(s.pending), pendingHead: s.pendingHead,
		equalityLimit:   len(s.equalityTrail),
		assignmentLimit: len(s.assignmentTrail), clauseLimit: len(s.clauses), justificationLimit: len(s.ownedJustifications), theoryLimit: len(s.theoryTrail), inconsistent: s.inconsistent,
		assignedLimit: len(s.assignedLiterals), propagationHead: s.propagationHead, conflict: s.conflict,
		conflictLiteral: s.conflictLiteral,
		disequalities:   disequalities, disequalityReasons: reasons,
		theoryEqLimit: len(s.theoryEqualities), theoryEqHead: s.theoryEqualityHead,
		theoryDiseqLimit: len(s.theoryDisequalities), theoryDiseqHead: s.theoryDisequalityHead,
		atomHead: s.atomPropagationHead, notifyLimit: len(s.notifyTrail), simplifyHead: s.simplifyHead,
		enodeFlagLimit:    len(s.enodeFlagTrail),
		atomNotifiedLimit: len(s.atomNotifiedTrail),
	})
	if s.quantifiers != nil {
		if err := s.quantifiers.PushScope(); err != nil {
			s.scopes = s.scopes[:len(s.scopes)-1]
			return err
		}
	}
	if s.quantifierManager != nil {
		if err := s.quantifierManager.PushScope(); err != nil {
			_ = s.quantifiers.PopScopes(1)
			s.scopes = s.scopes[:len(s.scopes)-1]
			return err
		}
	}
	if s.relevancy != nil {
		if err := s.relevancy.PushScope(); err != nil {
			if s.quantifierManager != nil {
				_ = s.quantifierManager.PopScopes(1)
			}
			if s.quantifiers != nil {
				_ = s.quantifiers.PopScopes(1)
			}
			s.scopes = s.scopes[:len(s.scopes)-1]
			return err
		}
	}
	if err := s.traceCaseSplitScope(false); err != nil {
		return err
	}
	for index, theory := range s.theoryOrder {
		if err := theory.PushScope(); err != nil {
			for undo := index - 1; undo >= 0; undo-- {
				_ = s.theoryOrder[undo].PopScopes(1)
			}
			if s.relevancy != nil {
				_ = s.relevancy.PopScopes(1)
			}
			if s.quantifierManager != nil {
				_ = s.quantifierManager.PopScopes(1)
			}
			if s.quantifiers != nil {
				_ = s.quantifiers.PopScopes(1)
			}
			s.scopes = s.scopes[:len(s.scopes)-1]
			return err
		}
	}
	s.scopeLevel++
	return nil
}

func (s *SMTInternalizerState) unregisterEnode(node *SMTEnode) error {
	if s.context.enodes[node.owner] != node {
		return fmt.Errorf("enode owner %d is not current: %w", node.owner.Slot(), ast.ErrASTDomain)
	}
	bucket := s.context.byDeclaration[node.declaration]
	if len(bucket) == 0 || bucket[len(bucket)-1] != node {
		return fmt.Errorf("enode declaration %d is not bucket tail (size %d): %w", node.declaration.Slot(), len(bucket), ast.ErrASTDomain)
	}
	if node.IsCongruenceRoot() && node.NumArgs() != 0 && node.congruenceEnabled && !node.IsTrueEquality() {
		erased, err := s.congruence.Erase(node)
		if err != nil || !erased {
			if err != nil {
				return fmt.Errorf("erase congruence entry: %w", err)
			}
			return fmt.Errorf("congruence entry was absent: %w", ast.ErrASTDomain)
		}
	}
	delete(s.context.enodes, node.owner)
	s.context.byDeclaration[node.declaration] = bucket[:len(bucket)-1]
	// Source relevancy marks belong to the expression's relevancy scope.
	// A base mark can precede an enode created only in this retiring scope.
	if s.relevancy == nil {
		delete(s.context.relevant, node.owner)
	}
	if err := node.Delete(true); err != nil {
		return fmt.Errorf("delete enode: %w", err)
	}
	if err := s.manager.DecRef(node.owner); err != nil {
		return fmt.Errorf("release enode owner: %w", err)
	}
	s.statistics.NumDelEnode++
	return nil
}

func (s *SMTInternalizerState) popTo(scope smtInternalizerScope) error {
	if err := s.relevanceGoal.restore(scope.goalQueue, s.caseSplitStrategy == 5); err != nil {
		return err
	}
	if scope.relActLimit < 0 || scope.relActLimit > len(s.relevanceActivity.entries) || scope.relActHead < 0 {
		return ast.ErrASTDomain
	}
	clear(s.relevanceActivity.entries[scope.relActLimit:])
	s.relevanceActivity.entries = s.relevanceActivity.entries[:scope.relActLimit]
	s.relevanceActivity.head = scope.relActHead
	if s.caseSplitStrategy == 4 {
		if err := smtCaseSplitAssert(591, "m_head <= m_queue.size()", func() bool { return s.relevanceActivity.head <= len(s.relevanceActivity.entries) }); err != nil {
			return err
		}
	}
	if s.relevanceActivity.head > len(s.relevanceActivity.entries) {
		return ast.ErrASTDomain
	}
	if err := s.relevanceFIFO.restore(scope.relevanceFIFO, s.caseSplitStrategy == 3); err != nil {
		return err
	}
	if err := s.traceCaseSplitScope(true); err != nil {
		return err
	}
	if err := s.undoTheoryCaseSplits(scope.theoryCaseSplitLimit); err != nil {
		return err
	}
	if scope.trueFirstLimit < 0 || scope.trueFirstLimit > len(s.trueFirstTrail) {
		return ast.ErrASTDomain
	}
	for len(s.trueFirstTrail) > scope.trueFirstLimit {
		i := len(s.trueFirstTrail) - 1
		s.booleanData[s.trueFirstTrail[i]].ResetTrueFirst()
		s.trueFirstTrail = s.trueFirstTrail[:i]
	}
	if scope.equalityLimit < 0 || scope.equalityLimit > len(s.equalityTrail) {
		return ast.ErrASTDomain
	}
	equalitiesRestored := len(s.equalityTrail) > scope.equalityLimit
	for len(s.equalityTrail) > scope.equalityLimit {
		index := len(s.equalityTrail) - 1
		entry := s.equalityTrail[index]
		s.equalityTrail = s.equalityTrail[:index]
		if err := s.restoreEqualitySnapshot(entry); err != nil {
			return fmt.Errorf("restore equality snapshot: %w", err)
		}
	}
	if equalitiesRestored {
		if err := s.rebuildCongruence(); err != nil {
			return fmt.Errorf("restore congruence index: %w", err)
		}
	}
	if scope.disequalities == nil {
		s.context.disequalities = make(map[smtEnodePairKey]struct{})
		s.disequalityReasons = make(map[smtEnodePairKey]SMTLiteral)
	} else {
		s.context.disequalities = scope.disequalities
		s.disequalityReasons = scope.disequalityReasons
	}
	if scope.theoryLimit < 0 || scope.theoryLimit > len(s.theoryTrail) {
		return ast.ErrASTDomain
	}
	for len(s.theoryTrail) > scope.theoryLimit {
		index := len(s.theoryTrail) - 1
		entry := s.theoryTrail[index]
		s.theoryTrail = s.theoryTrail[:index]
		if entry.old == SMTNullTheoryVar {
			if err := entry.node.DeleteTheoryVar(entry.theory); err != nil {
				return fmt.Errorf("delete node theory variable: %w", err)
			}
			if entry.rootAdded {
				if err := entry.root.DeleteTheoryVar(entry.theory); err != nil {
					return fmt.Errorf("delete root theory variable: %w", err)
				}
			}
		} else if err := entry.node.ReplaceTheoryVar(entry.theory, entry.old); err != nil {
			return fmt.Errorf("restore node theory variable: %w", err)
		}
	}
	if scope.assignmentLimit < 0 || scope.assignmentLimit > len(s.assignmentTrail) {
		return ast.ErrASTDomain
	}
	for len(s.assignmentTrail) > scope.assignmentLimit {
		entry := s.assignmentTrail[len(s.assignmentTrail)-1]
		s.assignmentTrail = s.assignmentTrail[:len(s.assignmentTrail)-1]
		positive := NewSMTLiteral(entry.variable, false)
		s.assignment[positive.Index()] = entry.value
		s.assignment[positive.Negated().Index()] = entry.value.Negated()
		s.booleanData[entry.variable].SetJustification(entry.justification)
		s.booleanData[entry.variable].ScopeLevel = entry.scopeLevel
		if entry.value == lbool.Undef {
			if err := s.basicActivityUnassigned(entry.variable); err != nil {
				return err
			}
			if err := s.relevanceActivityUnassigned(entry.variable); err != nil {
				return err
			}
		}
		if entry.value == lbool.Undef && s.theoryAwareBranching != nil {
			if err := s.theoryAwareBranching.unassigned(entry.variable); err != nil {
				return err
			}
		}
		if err := s.context.SetAssignment(s.boolExpr[entry.variable], entry.value); err != nil {
			return fmt.Errorf("restore Boolean assignment: %w", err)
		}
	}
	if scope.assignedLimit < 0 || scope.assignedLimit > len(s.assignedLiterals) {
		return ast.ErrASTDomain
	}
	s.assignedLiterals = s.assignedLiterals[:scope.assignedLimit]
	if scope.propagationHead < 0 || scope.propagationHead > len(s.assignedLiterals) {
		return ast.ErrASTDomain
	}
	s.propagationHead = scope.propagationHead
	s.simplifyHead = scope.simplifyHead
	if scope.theoryEqLimit < 0 || scope.theoryEqLimit > len(s.theoryEqualities) || scope.theoryEqHead < 0 || scope.theoryEqHead > scope.theoryEqLimit {
		return ast.ErrASTDomain
	}
	s.theoryEqualities = s.theoryEqualities[:scope.theoryEqLimit]
	s.theoryEqualityHead = scope.theoryEqHead
	if scope.theoryDiseqLimit < 0 || scope.theoryDiseqLimit > len(s.theoryDisequalities) || scope.theoryDiseqHead < 0 || scope.theoryDiseqHead > scope.theoryDiseqLimit {
		return ast.ErrASTDomain
	}
	s.theoryDisequalities = s.theoryDisequalities[:scope.theoryDiseqLimit]
	s.theoryDisequalityHead = scope.theoryDiseqHead
	if scope.atomHead < 0 || scope.atomHead > len(s.assignedLiterals) || scope.notifyLimit < 0 || scope.notifyLimit > len(s.notifyTrail) {
		return ast.ErrASTDomain
	}
	s.atomPropagationHead = scope.atomHead
	if scope.atomNotifiedLimit < 0 || scope.atomNotifiedLimit > len(s.atomNotifiedTrail) {
		return ast.ErrASTDomain
	}
	for len(s.atomNotifiedTrail) > scope.atomNotifiedLimit {
		i := len(s.atomNotifiedTrail) - 1
		delete(s.atomNotified, s.atomNotifiedTrail[i])
		s.atomNotifiedTrail = s.atomNotifiedTrail[:i]
	}
	if scope.enodeFlagLimit < 0 || scope.enodeFlagLimit > len(s.enodeFlagTrail) {
		return ast.ErrASTDomain
	}
	for len(s.enodeFlagTrail) > scope.enodeFlagLimit {
		i := len(s.enodeFlagTrail) - 1
		s.booleanData[s.enodeFlagTrail[i]].ResetENode()
		s.enodeFlagTrail = s.enodeFlagTrail[:i]
	}
	for len(s.notifyTrail) > scope.notifyLimit {
		index := len(s.notifyTrail) - 1
		entry := s.notifyTrail[index]
		s.notifyTrail = s.notifyTrail[:index]
		s.booleanData[entry.variable].ResetNotifyTheory()
		if entry.theory != SMTNullTheoryID {
			if err := s.booleanData[entry.variable].SetNotifyTheory(entry.theory); err != nil {
				return err
			}
		}
	}
	if scope.clauseLimit < 0 || scope.clauseLimit > len(s.clauses) {
		return ast.ErrASTDomain
	}
	for index := len(s.clauses); index > scope.clauseLimit; index-- {
		if err := s.closeClause(&s.clauses[index-1]); err != nil {
			return fmt.Errorf("close clause: %w", err)
		}
	}
	s.statistics.NumDelClause += uint32(len(s.clauses) - scope.clauseLimit)
	s.clauses = s.clauses[:scope.clauseLimit]
	if scope.justificationLimit < 0 || scope.justificationLimit > len(s.ownedJustifications) {
		return ast.ErrASTDomain
	}
	for len(s.ownedJustifications) > scope.justificationLimit {
		index := len(s.ownedJustifications) - 1
		if err := s.ownedJustifications[index].Release(s.manager); err != nil {
			return fmt.Errorf("release owned justification: %w", err)
		}
		s.ownedJustifications = s.ownedJustifications[:index]
	}
	for len(s.enodeStack) > scope.enodeLimit {
		node := s.enodeStack[len(s.enodeStack)-1]
		if err := s.unregisterEnode(node); err != nil {
			return fmt.Errorf("unregister enode: %w", err)
		}
		s.enodeStack = s.enodeStack[:len(s.enodeStack)-1]
	}
	for len(s.booleanStack) > scope.booleanLimit {
		expression := s.booleanStack[len(s.booleanStack)-1]
		variable := s.boolVar[expression]
		if (s.cachedActivity || s.caseSplitStrategy == 2) && s.searching && s.activity[variable] > 0 {
			if err := s.cacheBooleanActivity(expression, s.activity[variable]); err != nil {
				return err
			}
		}
		if err := s.basicActivityDeleted(variable); err != nil {
			return err
		}
		if err := s.relevanceActivityDeleted(variable); err != nil {
			return err
		}
		if s.theoryAwareBranching != nil {
			if err := s.theoryAwareBranching.deleted(variable); err != nil {
				return err
			}
		}
		if s.quantifiers != nil {
			kind, err := s.manager.GetKind(expression)
			if err != nil {
				return err
			}
			if kind == ast.ASTQuantifier {
				if _, statErr := s.quantifiers.Stat(expression); statErr == nil {
					if err = s.quantifiers.Delete(expression); err != nil {
						return err
					}
				} else if !errors.Is(statErr, ast.ErrASTDomain) {
					return statErr
				}
			}
		}
		delete(s.boolVar, expression)
		delete(s.context.assignments, expression)
		if s.relevancy == nil {
			delete(s.context.relevant, expression)
		}
		s.boolExpr[variable] = ast.ASTHandle{}
		s.boolExpr = s.boolExpr[:variable]
		s.booleanData = s.booleanData[:variable]
		s.booleanCreation = s.booleanCreation[:variable]
		s.activity = s.activity[:variable]
		s.assignment = s.assignment[:2*variable]
		s.watches = s.watches[:2*variable]
		s.booleanStack = s.booleanStack[:len(s.booleanStack)-1]
		if err := s.manager.DecRef(expression); err != nil {
			return fmt.Errorf("release Boolean expression: %w", err)
		}
		s.statistics.NumDelBoolVar++
	}
	if scope.pendingLimit < 0 || scope.pendingLimit > len(s.pending) {
		return ast.ErrASTDomain
	}
	clear(s.pending[scope.pendingLimit:])
	s.pending = s.pending[:scope.pendingLimit]
	if scope.pendingHead < 0 || scope.pendingHead > len(s.pending) {
		return ast.ErrASTDomain
	}
	s.pendingHead = scope.pendingHead
	s.inconsistent = scope.inconsistent
	s.conflict = scope.conflict
	s.conflictLiteral = scope.conflictLiteral
	return nil
}

func (s *SMTInternalizerState) PopScopes(count uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	if err := s.assertCaseSplitPopCount(count); err != nil {
		return err
	}
	if count == 0 || count > uint32(len(s.scopes)) {
		return ast.ErrASTDomain
	}
	level := len(s.scopes) - int(count)
	preserve := s.searching && uint32(level) >= s.searchBaseLevel
	var learned []smtPreservedLearnedClause
	var err error
	if preserve {
		learned, err = s.preserveLearnedClauses(s.scopes[level].clauseLimit)
		if err != nil {
			return err
		}
	}
	if err := s.popTo(s.scopes[level]); err != nil {
		return errors.Join(err, s.releasePreservedLearnedClauses(learned))
	}
	if s.relevancy != nil {
		if err := s.relevancy.PopScopes(count); err != nil {
			return errors.Join(err, s.releasePreservedLearnedClauses(learned))
		}
	}
	for index := len(s.theoryOrder) - 1; index >= 0; index-- {
		if err := s.theoryOrder[index].PopScopes(count); err != nil {
			return errors.Join(err, s.releasePreservedLearnedClauses(learned))
		}
	}
	if s.quantifierManager != nil {
		if err := s.quantifierManager.PopScopes(count); err != nil {
			return errors.Join(err, s.releasePreservedLearnedClauses(learned))
		}
	}
	if s.quantifiers != nil {
		if err := s.quantifiers.PopScopes(count); err != nil {
			return errors.Join(err, s.releasePreservedLearnedClauses(learned))
		}
	}
	s.scopes = s.scopes[:level]
	s.scopeLevel = uint32(level)
	if preserve {
		if err = s.restoreLearnedClauses(learned); err != nil {
			return err
		}
		return s.restoreLearnedUnits(uint32(level))
	}
	return s.discardLearnedUnitsAbove(uint32(level))
}

func (s *SMTInternalizerState) Statistics() SMTContextStatistics {
	if s == nil {
		return SMTContextStatistics{}
	}
	return s.statistics
}

func (s *SMTInternalizerState) Close() error {
	if s == nil || s.manager == nil {
		return ast.ErrASTHandle
	}
	if s.closed {
		return nil
	}
	var result error
	if s.lemmaProblem != nil {
		if err := s.lemmaProblem.close(); err != nil {
			result = errors.Join(result, err)
		} else {
			s.lemmaProblem = nil
		}
	}
	if len(s.scopes) != 0 {
		if err := s.PopScopes(uint32(len(s.scopes))); err != nil {
			result = errors.Join(result, fmt.Errorf("close SMT scopes: %w", err))
		}
	}
	if err := s.popTo(smtInternalizerScope{}); err != nil {
		result = errors.Join(result, fmt.Errorf("close SMT root scope: %w", err))
	}
	for i := len(s.theoryOrder) - 1; i >= 0; i-- {
		if closer, ok := s.theoryOrder[i].(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				result = errors.Join(result, fmt.Errorf("close SMT theory %T: %w", s.theoryOrder[i], err))
			}
		}
	}
	result = errors.Join(result, s.releaseLearnedUnits())
	result = errors.Join(result, s.releaseBooleanActivityCache())
	if s.theoryAwareBranching != nil {
		result = errors.Join(result, s.theoryAwareBranching.heap.Close())
		s.theoryAwareBranching = nil
	}
	if s.delayedActivity != nil {
		if err := s.delayedActivity.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			s.delayedActivity = nil
		}
	}
	if s.basicActivity != nil {
		if err := s.basicActivity.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			s.basicActivity = nil
		}
	}
	if s.relevanceGoal.heap != nil {
		if err := s.relevanceGoal.heap.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			s.relevanceGoal.heap = nil
		}
	}
	if s.relevanceActivity.delayed != nil {
		if err := s.relevanceActivity.delayed.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			s.relevanceActivity.delayed = nil
		}
	}
	result = errors.Join(result, s.dynamicAck.close())
	if s.quantifierManager != nil {
		result = errors.Join(result, s.quantifierManager.Close())
	}
	if s.quantifiers != nil {
		result = errors.Join(result, s.quantifiers.schedule.Reset())
	}
	result = errors.Join(result, s.congruence.Close())
	if result == nil {
		if s.relevancy != nil && s.relevancy.internalizer == s {
			s.relevancy.internalizer = nil
		}
		s.closed = true
		s.theoryRewriter = nil
		s.smtParams = nil
		s.context = nil
		s.manager = nil
	}
	return result
}
