package smtcore

// Source: retained store/select axiom fragment of src/smt/theory_array_base.h,
// src/smt/theory_array_base.cpp, src/smt/theory_array.h, and
// src/smt/theory_array.cpp.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/backtrack"
	"github.com/glycerine/g3/pkg/util/statistics"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type smtArrayScope struct {
	stores, selects, defaults, epsilons int
	defaultFingerprints                 int
	unsupported                         bool
	extensionality                      int
	readFingerprints                    int
	sorts                               int
}

type smtArrayAxiomPair struct {
	observer ast.ASTHandle
	array    ast.ASTHandle
}

type smtArrayStatistics struct {
	axiom1, axiom2, expandedAxiom2, extensionality        uint32
	mapAxiom, defaultMap, selectConst, defaultStore       uint32
	defaultConst, defaultAsArray, selectAsArray, eqSplits uint32
}

// SMTArrayTheory internalizes read-over-write, constant-array, map, default,
// as-array, and extensionality axioms and constructs retained array models.
type SMTArrayTheory struct {
	*SMTTheoryBase
	stores, selects, defaults             map[ast.ASTHandle]bool
	storeTrail, selectTrail, defaultTrail []ast.ASTHandle
	defaultFingerprints                   map[*SMTEnode]bool
	defaultFingerprintTrail               []*SMTEnode
	equalityAxioms                        map[smtArrayExpressionPair]bool
	epsilons                              map[ast.ASTHandle]ast.ASTHandle // owns one reference per cached value
	epsilonTrail                          []ast.ASTHandle
	scopes                                []smtArrayScope
	unsupported                           bool
	statistics                            smtArrayStatistics
	extensional                           bool
	simple                                bool
	extensionalityFingerprints            map[smtArrayEnodePair]bool
	extensionalityTrail                   []smtArrayEnodePair
	extensionalityTodo                    []smtArrayEnodePair
	arrayConfig                           SMTArrayState
	finalCheckIndex                       uint32
	storeTodo                             []ast.ASTHandle
	readTodo                              []smtArrayAxiomPair
	readFingerprints                      map[ast.ASTHandle][]*smtArrayReadFingerprint
	readFingerprintTrail                  []*smtArrayReadFingerprint
	arrayUnion                            *backtrack.UnionFind
	arrayTrail                            *backtrack.TrailStack
	arrayData                             []*smtArrayVarData
	arrayMergeError                       error
	parameterOwner                        *SMTArrayParams
	schedulingReady                       bool
	modelData                             *smtArrayModelData
	sortDeclarations                      map[ast.ASTHandle][]ast.ASTHandle
	sortTrail                             []ast.ASTHandle
}

func NewSMTArrayTheory(family ast.FamilyID) (*SMTArrayTheory, error) {
	base, err := NewSMTTheoryBase(family)
	if err != nil {
		return nil, err
	}
	theory := &SMTArrayTheory{
		SMTTheoryBase:              base,
		extensional:                true,
		stores:                     make(map[ast.ASTHandle]bool),
		selects:                    make(map[ast.ASTHandle]bool),
		defaults:                   make(map[ast.ASTHandle]bool),
		equalityAxioms:             make(map[smtArrayExpressionPair]bool),
		defaultFingerprints:        make(map[*SMTEnode]bool),
		sortDeclarations:           make(map[ast.ASTHandle][]ast.ASTHandle),
		epsilons:                   make(map[ast.ASTHandle]ast.ASTHandle),
		extensionalityFingerprints: make(map[smtArrayEnodePair]bool),
		arrayConfig:                SMTArrayState{Mode: 3, Simplify: true, Extensional: true, Laziness: 1, DelayExpAxiom: true, AlwaysPropUpward: true, LazyIEqDelay: 10},
		readFingerprints:           make(map[ast.ASTHandle][]*smtArrayReadFingerprint),
		arrayTrail:                 backtrack.NewTrailStack(),
	}
	theory.arrayUnion, err = backtrack.NewUnionFind(&smtArrayUnionContext{theory})
	return theory, err
}

func (t *SMTArrayTheory) arrayOperation(expression ast.ASTHandle) (int32, bool, error) {
	state, err := t.State()
	if err != nil {
		return 0, false, err
	}
	kind, err := state.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return 0, false, err
	}
	family, err := state.manager.GetFamilyID(expression)
	if err != nil || family != t.FamilyID() {
		return 0, false, err
	}
	op, err := state.manager.GetDeclKind(expression)
	return op, err == nil, err
}

func (t *SMTArrayTheory) InternalizeAtom(expression ast.ASTHandle, _ bool) (bool, error) {
	if op, array, err := t.arrayOperation(expression); err != nil {
		return false, err
	} else if array {
		if op == ast.ASTArrayOpExt && t.simple {
			return false, nil
		}
		if t.simple && op != ast.ASTArrayOpSelect && op != ast.ASTArrayOpStore {
			if err := t.foundUnsupportedArrayOperation(expression); err != nil {
				return false, err
			}
			return false, nil
		}
		if op == ast.ASTArrayOpSelect || op == ast.ASTArrayOpDefault || op == ast.ASTArrayOpExt {
			_, termErr := t.InternalizeTerm(expression)
			if termErr != nil {
				return false, fmt.Errorf("array select atom: %w", termErr)
			}
			state, _ := t.State()
			variable, found := state.boolVar[expression]
			if !found {
				variable, termErr = state.MkBoolVar(expression)
				if termErr != nil {
					return false, fmt.Errorf("array select Boolean variable: %w", termErr)
				}
			}
			data, dataErr := state.BooleanData(variable)
			if dataErr != nil {
				return false, dataErr
			}
			if !data.IsTheoryAtom() {
				if dataErr = state.SetVariableTheory(variable, t.FamilyID()); dataErr != nil {
					return false, fmt.Errorf("array select Boolean theory: %w", dataErr)
				}
			}
			// Source internalize_term_core explicitly marks Boolean array
			// terms as having enodes, even when they occur in gate context.
			if err := state.ensureBooleanEnode(expression, variable, false, false, true); err != nil {
				return false, err
			}
			return true, nil
		}
		if err := t.foundUnsupportedArrayOperation(expression); err != nil {
			return false, err
		}
	}
	return false, nil
}

func (t *SMTArrayTheory) InternalizeTerm(expression ast.ASTHandle) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	if !t.simple {
		if err = t.traceFullArrayExpression("internalize_term", 260, expression); err != nil {
			return false, err
		}
	}
	if err = t.traceSimpleArrayExpression("array_bug", "internalize_term", 267, "", expression); err != nil {
		return false, err
	}
	op, array, err := t.arrayOperation(expression)
	if err != nil || !array {
		return false, err
	}
	if op == ast.ASTArrayOpExt && t.simple {
		return false, nil
	}
	if t.simple && op != ast.ASTArrayOpStore && op != ast.ASTArrayOpSelect {
		if err = t.foundUnsupportedArrayOperation(expression); err != nil {
			return false, err
		}
		return false, nil
	}
	if err = t.traceSimpleArrayExpression("array_bug", "internalize_term_core", 241, "", expression); err != nil {
		return false, err
	}
	count, err := state.manager.GetNumArgs(expression)
	if err != nil {
		return false, err
	}
	for index := uint32(0); index < count; index++ {
		argument, argErr := state.manager.GetArg(expression, index)
		if argErr != nil {
			return false, argErr
		}
		if argErr = state.Internalize(argument, false); argErr != nil {
			return false, fmt.Errorf("array term argument %d: %w", index, argErr)
		}
	}
	handled := op == ast.ASTArrayOpStore || op == ast.ASTArrayOpSelect || op == ast.ASTArrayOpConst || op == ast.ASTArrayOpExt || op == ast.ASTArrayOpMap || op == ast.ASTArrayOpAsArray || op == ast.ASTArrayOpDefault
	if handled {
		node := state.context.enodes[expression]
		if node != nil && t.IsAttachedToVariable(node) {
			return true, nil
		}
		if node == nil {
			node, err = state.MkEnode(expression, false, false, true)
			if err != nil {
				return false, err
			}
		}
		if !t.IsAttachedToVariable(node) {
			if _, err = t.CreateVariable(node); err != nil {
				return false, err
			}
		}
	}
	switch op {
	case ast.ASTArrayOpStore:
		if err = t.assertStoreAxiom(expression); err != nil {
			return false, fmt.Errorf("array store axiom: %w", err)
		}
		if t.arrayConfig.Laziness == 0 {
			base := state.context.enodes[expression].arguments[0]
			if err = arraySourceAssert(280, "v_arg != null_theory_var", func() bool { return base.TheoryVar(t.FamilyID()) != SMTNullTheoryVar }); err != nil {
				return false, err
			}
			if err = t.addParentStore(base.TheoryVar(t.FamilyID()), state.context.enodes[expression]); err != nil {
				return false, err
			}
		}
	case ast.ASTArrayOpSelect:
		if err = t.assertSelectStoreAxioms(expression); err != nil {
			return false, fmt.Errorf("array select-store axiom: %w", err)
		}
	case ast.ASTArrayOpConst:
	case ast.ASTArrayOpExt:
		node := state.context.enodes[expression]
		if len(node.arguments) != 2 {
			return false, ast.ErrASTDomain
		}
		if err = t.instantiateArrayExtensionality(node.arguments[0], node.arguments[1]); err != nil {
			return false, err
		}
	case ast.ASTArrayOpMap:
		for _, input := range state.context.enodes[expression].arguments {
			if err = t.addParentMap(input.TheoryVar(t.FamilyID()), state.context.enodes[expression]); err != nil {
				return false, err
			}
		}
	case ast.ASTArrayOpAsArray:
		if err = t.foundUnsupportedArrayOperation(expression); err != nil {
			return false, err
		}
	case ast.ASTArrayOpDefault:
		if err = t.assertDefaultAxiom(expression); err != nil {
			return false, fmt.Errorf("array default axiom: %w", err)
		}
	default:
		if err = t.foundUnsupportedArrayOperation(expression); err != nil {
			return false, err
		}
	}
	return handled, nil
}

func (t *SMTArrayTheory) assertStoreAxiom(store ast.ASTHandle) error {
	if t.stores[store] {
		return nil
	}
	op, array, err := t.arrayOperation(store)
	if err != nil {
		return err
	}
	if err = arraySourceAssert(201, "is_store(store)", func() bool { return array && op == ast.ASTArrayOpStore }); err != nil {
		return err
	}
	if !array || op != ast.ASTArrayOpStore {
		return ast.ErrASTDomain
	}
	if err := t.traceSimpleArrayExpression("array", "instantiate_axiom1", 200, "axiom 1:\n", store); err != nil {
		return err
	}
	t.stores[store] = true
	t.storeTrail = append(t.storeTrail, store)
	t.storeTodo = append(t.storeTodo, store)
	t.statistics.axiom1++
	return nil
}

func (t *SMTArrayTheory) assertStoreAxiomCore(store ast.ASTHandle) error {
	state, _ := t.State()
	count, err := state.manager.GetNumArgs(store)
	if err != nil || count < 3 {
		return err
	}
	arguments := make([]ast.ASTHandle, 0, count-1)
	arguments = append(arguments, store)
	for index := uint32(1); index+1 < count; index++ {
		argument, argErr := state.manager.GetArg(store, index)
		if argErr != nil {
			return argErr
		}
		arguments = append(arguments, argument)
	}
	selected, err := t.makeArraySelect(arguments)
	if err != nil {
		return err
	}
	value, err := state.manager.GetArg(store, count-1)
	if err != nil {
		return err
	}
	if err = t.traceArrayExpressions("array", "assert_store_axiom1_core", 106, "", selected, value); err != nil {
		return err
	}
	proofs, err := state.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	if proofs {
		literal, err := t.MkEqualityLiteral(selected, value, true)
		if err != nil {
			return err
		}
		if literal.Var() >= SMTFirstBoolVar {
			if err = state.markRelevant(state.boolExpr[literal.Var()]); err != nil {
				return err
			}
		}
		return t.assertArrayAxiom([]SMTLiteral{literal})
	}
	if err = t.emitArrayTrace("mk_var_bug", "assert_store_axiom1_core", 113, nil, func(out io.Writer) error {
		id, err := state.manager.GetASTID(selected)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "mk_sel: %d\n", id)
		return err
	}); err != nil {
		return err
	}
	if err = state.Internalize(selected, false); err != nil {
		return err
	}
	if err = state.assignEquality(state.context.enodes[selected], state.context.enodes[value], NewSMTEqualityAxiom()); err != nil {
		return err
	}
	return state.markRelevant(selected)
}

func (t *SMTArrayTheory) assertSelectStoreAxioms(selectExpression ast.ASTHandle) error {
	if !t.selects[selectExpression] {
		t.selects[selectExpression] = true
		t.selectTrail = append(t.selectTrail, selectExpression)
	}
	state, _ := t.State()
	array, err := state.manager.GetArg(selectExpression, 0)
	if err != nil {
		return err
	}
	if t.arrayConfig.Laziness == 0 {
		return t.addParentSelect(state.context.enodes[array].TheoryVar(t.FamilyID()), state.context.enodes[selectExpression])
	}
	return nil
}

func (t *SMTArrayTheory) assertSelectAxiom(selectExpression, array ast.ASTHandle) error {
	op, isArray, err := t.arrayOperation(array)
	if err != nil || !isArray {
		return err
	}
	if op == ast.ASTArrayOpStore {
		return t.enqueueStoreReadAxiom(selectExpression, array)
	}
	if t.simple {
		return nil
	}
	_, err = t.instantiateConstructorRead(selectExpression, array)
	return err
}

func (t *SMTArrayTheory) assertDefaultAxiom(defaultExpression ast.ASTHandle) error {
	if !t.defaults[defaultExpression] {
		t.defaults[defaultExpression] = true
		t.defaultTrail = append(t.defaultTrail, defaultExpression)
	}
	state, err := t.State()
	if err != nil {
		return err
	}
	array, err := state.manager.GetArg(defaultExpression, 0)
	if err != nil {
		return err
	}
	node := state.context.enodes[array]
	if node == nil {
		return ast.ErrASTDomain
	}
	variable := node.TheoryVar(t.FamilyID())
	if variable < 0 {
		return ast.ErrASTDomain
	}
	// Retained add_parent_default visits stores in the private array class.
	data := t.arrayData[t.arrayUnion.Find(uint32(variable))]
	for _, store := range data.stores {
		if _, err := t.instantiateConstructorDefault(store.owner); err != nil {
			return err
		}
	}
	return nil
}

func (t *SMTArrayTheory) arrayEpsilon(sort ast.ASTHandle) (ast.ASTHandle, error) {
	if epsilon := t.epsilons[sort]; !epsilon.IsNull() {
		return epsilon, nil
	}
	state, _ := t.State()
	epsilon, err := state.manager.MkFreshConst("epsilon", sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	// Source m_sort2epsilon is an owning ast2ast_trailmap. Keep the fresh value
	// alive independently of temporary expressions and their references.
	if err = state.manager.IncRef(epsilon); err != nil {
		return ast.ASTHandle{}, err
	}
	t.epsilons[sort] = epsilon
	t.epsilonTrail = append(t.epsilonTrail, sort)
	return epsilon, nil
}

func (t *SMTArrayTheory) arrayHasLargeDomain(array ast.ASTHandle) (bool, error) {
	state, _ := t.State()
	sort, err := state.manager.GetSort(array)
	if err != nil {
		return false, err
	}
	cardinality := uint64(1)
	for index := uint32(0); ; index++ {
		declaration, declErr := state.manager.MkArrayExtDecl(sort, index)
		if errors.Is(declErr, ast.ErrASTDomain) {
			break
		}
		if declErr != nil {
			return false, declErr
		}
		domain, rangeErr := state.manager.GetRange(declaration)
		if rangeErr != nil {
			return false, rangeErr
		}
		size, sizeErr := state.manager.GetSortSize(domain)
		if sizeErr != nil {
			return false, sizeErr
		}
		if size.IsInfinite() || size.IsVeryBig() {
			return true, nil
		}
		value, valueErr := size.Size()
		if valueErr != nil {
			return false, valueErr
		}
		if value == 0 {
			return false, nil
		}
		if cardinality >= 1<<20 || value >= (1<<20+cardinality-1)/cardinality {
			return true, nil
		}
		cardinality *= value
	}
	return cardinality >= 1<<20, nil
}

func (*SMTArrayTheory) InternalizeEquality(ast.ASTHandle, int32) error { return nil }

func (t *SMTArrayTheory) assertExtensionalityCore(left, right ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	sort, err := state.manager.GetSort(left)
	if err != nil {
		return err
	}
	leftArguments := []ast.ASTHandle{left}
	rightArguments := []ast.ASTHandle{right}
	declarations := t.sortDeclarations[sort]
	if len(declarations) == 0 {
		return ast.ErrASTDomain
	}
	for _, declaration := range declarations {
		witness, appErr := state.manager.MkApp(declaration, []ast.ASTHandle{left, right})
		if appErr != nil {
			return appErr
		}
		leftArguments = append(leftArguments, witness)
		rightArguments = append(rightArguments, witness)
	}
	if len(leftArguments) == 1 {
		return ast.ErrASTDomain
	}
	leftSelect, err := t.makeArraySelect(leftArguments)
	if err != nil {
		return err
	}
	rightSelect, err := t.makeArraySelect(rightArguments)
	if err != nil {
		return err
	}
	if err = t.emitArrayTrace("ext", "assert_extensionality_core", 325, nil, func(out io.Writer) error {
		if err := displayArrayTraceExpression(state.manager, out, leftSelect); err != nil {
			return err
		}
		if _, err := io.WriteString(out, "\n"); err != nil {
			return err
		}
		if err := displayArrayTraceExpression(state.manager, out, rightSelect); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	}); err != nil {
		return err
	}
	arrayEquality, err := t.MkEqualityLiteral(left, right, true)
	if err != nil {
		return err
	}
	selectEquality, err := t.MkEqualityLiteral(leftSelect, rightSelect, true)
	if err != nil {
		return err
	}
	// assert_extensionality_core marks both equality atoms relevant before
	// adding the witness axiom, so its reads constrain the published model.
	for _, literal := range []SMTLiteral{arrayEquality, selectEquality} {
		if literal.Var() >= SMTFirstBoolVar {
			if err := state.markRelevant(state.boolExpr[literal.Var()]); err != nil {
				return err
			}
		}
	}
	if err = t.assertArrayAxiom([]SMTLiteral{arrayEquality, selectEquality.Negated()}); err != nil {
		return err
	}
	return nil
}
func (t *SMTArrayTheory) NewEquality(left, right SMTTheoryVar) error {
	if _, err := t.Enode(left); err != nil {
		return err
	}
	if _, err := t.Enode(right); err != nil {
		return err
	}
	t.arrayUnion.Merge(uint32(left), uint32(right))
	if t.arrayMergeError != nil {
		return t.arrayMergeError
	}
	return nil
}
func (*SMTArrayTheory) Assign(int32, bool) error { return nil }

func (t *SMTArrayTheory) ApplySortConstraint(node *SMTEnode, sort ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	family, err := state.manager.GetFamilyID(sort)
	if err != nil {
		return err
	}
	if err = arraySourceAssert(293, "is_array_sort(s)", func() bool { return family == t.FamilyID() }); err != nil {
		return err
	}
	if family != t.FamilyID() || t.IsAttachedToVariable(node) {
		return err
	}
	_, err = t.CreateVariable(node)
	return err
}

func (t *SMTArrayTheory) PushScope() error {
	if err := t.SMTTheoryBase.PushScope(); err != nil {
		return err
	}
	t.arrayTrail.PushScope()
	t.scopes = append(t.scopes, smtArrayScope{
		stores:              len(t.storeTrail),
		selects:             len(t.selectTrail),
		defaults:            len(t.defaultTrail),
		epsilons:            len(t.epsilonTrail),
		defaultFingerprints: len(t.defaultFingerprintTrail),
		unsupported:         t.unsupported,
		extensionality:      len(t.extensionalityTrail),
		readFingerprints:    len(t.readFingerprintTrail),
		sorts:               len(t.sortTrail),
	})
	return nil
}

func (t *SMTArrayTheory) PopScopes(count uint32) error {
	if count == 0 || count > uint32(len(t.scopes)) {
		return ast.ErrASTDomain
	}
	level := len(t.scopes) - int(count)
	scope := t.scopes[level]
	oldVars, err := t.OldNumVariables(count)
	if err != nil {
		return err
	}
	if err := t.arrayTrail.PopScope(count); err != nil {
		return err
	}
	t.arrayData = t.arrayData[:oldVars]
	// Source theory_array_base::pop_scope_eh clears pending work before
	// dropping the enodes and fingerprints owned by the popped scopes.
	t.extensionalityTodo = nil
	t.storeTodo, t.readTodo = nil, nil
	if err := t.restoreArraySorts(scope.sorts); err != nil {
		return err
	}
	// theory_array_full::pop_scope_eh resets the entire ordered-pair cache,
	// including pairs inserted before the popped scopes.
	clear(t.equalityAxioms)
	for i := len(t.readFingerprintTrail) - 1; i >= scope.readFingerprints; i-- {
		entry := t.readFingerprintTrail[i]
		bucket := t.readFingerprints[entry.store]
		bucket = bucket[:len(bucket)-1]
		if len(bucket) == 0 {
			delete(t.readFingerprints, entry.store)
		} else {
			t.readFingerprints[entry.store] = bucket
		}
	}
	t.readFingerprintTrail = t.readFingerprintTrail[:scope.readFingerprints]
	for _, pair := range t.extensionalityTrail[scope.extensionality:] {
		delete(t.extensionalityFingerprints, pair)
	}
	t.extensionalityTrail = t.extensionalityTrail[:scope.extensionality]
	for _, expression := range t.storeTrail[scope.stores:] {
		delete(t.stores, expression)
	}
	for _, expression := range t.selectTrail[scope.selects:] {
		delete(t.selects, expression)
	}
	for _, expression := range t.defaultTrail[scope.defaults:] {
		delete(t.defaults, expression)
	}
	if err := t.restoreArrayEpsilons(scope.epsilons); err != nil {
		return err
	}
	for _, node := range t.defaultFingerprintTrail[scope.defaultFingerprints:] {
		delete(t.defaultFingerprints, node)
	}
	t.storeTrail = t.storeTrail[:scope.stores]
	t.selectTrail = t.selectTrail[:scope.selects]
	t.defaultTrail = t.defaultTrail[:scope.defaults]
	t.defaultFingerprintTrail = t.defaultFingerprintTrail[:scope.defaultFingerprints]
	t.unsupported = scope.unsupported
	t.scopes = t.scopes[:level]
	if err = t.SMTTheoryBase.PopScopes(count); err != nil {
		return err
	}
	if err = arraySourceAssert(348, "m_find.get_num_vars() == m_var_data.size()", func() bool { return t.arrayUnion.GetNumVars() == uint32(len(t.arrayData)) }); err != nil {
		return err
	}
	return arraySourceAssert(349, "m_find.get_num_vars() == get_num_vars()", func() bool { return t.arrayUnion.GetNumVars() == uint32(t.NumVariables()) })
}

func (t *SMTArrayTheory) FinalCheck() (SMTFinalCheckStatus, error) {
	t.finalCheckIndex++
	var delayed, interfaces bool
	var err error
	if t.arrayConfig.LazyIEq && t.finalCheckIndex%t.arrayConfig.LazyIEqDelay != 0 {
		if _, err = t.assertDelayedStoreAxioms(); err != nil {
			return SMTFinalCheckGiveUp, err
		}
		// The native final-check loop revisits delayed theories. Record this
		// bounded scheduling step so Go's no-progress guard does so as well.
		t.state.theoryProgress++
		return SMTFinalCheckContinue, nil
	}
	if !t.arrayConfig.LazyIEq && t.finalCheckIndex%2 == 1 {
		delayed, err = t.assertDelayedStoreAxioms()
		if err == nil && !delayed {
			interfaces, err = t.makeInterfaceEqualities()
		}
	} else {
		interfaces, err = t.makeInterfaceEqualities()
		if err == nil && !interfaces {
			delayed, err = t.assertDelayedStoreAxioms()
		}
	}
	if err != nil {
		return SMTFinalCheckGiveUp, err
	}
	status := SMTFinalCheckDone
	if delayed || interfaces {
		status = SMTFinalCheckContinue
	} else if t.unsupported {
		status = SMTFinalCheckGiveUp
	}
	if err = t.emitSimpleArrayTrace("array", "final_check_eh", 385, nil, func(out io.Writer) error {
		_, writeErr := fmt.Fprintf(out, "m_found_unsupported_op: %t %d\n", t.unsupported, status)
		return writeErr
	}); err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if status == SMTFinalCheckContinue {
		return SMTFinalCheckContinue, nil
	}
	if status == SMTFinalCheckGiveUp {
		return SMTFinalCheckGiveUp, nil
	}
	return SMTFinalCheckDone, nil
}

func (*SMTArrayTheory) Name() string { return "array" }

func (t *SMTArrayTheory) CollectStatistics(target *statistics.Statistics) error {
	if t == nil || target == nil {
		return ast.ErrASTHandle
	}
	target.UpdateUint64("array ax1", uint64(t.statistics.axiom1))
	target.UpdateUint64("array ax2", uint64(t.statistics.axiom2))
	target.UpdateUint64("array exp ax2", uint64(t.statistics.expandedAxiom2))
	target.UpdateUint64("array ext ax", uint64(t.statistics.extensionality))
	target.UpdateUint64("array splits", uint64(t.statistics.eqSplits))
	target.UpdateUint64("array map ax", uint64(t.statistics.mapAxiom))
	target.UpdateUint64("array def const", uint64(t.statistics.defaultConst))
	target.UpdateUint64("array sel const", uint64(t.statistics.selectConst))
	target.UpdateUint64("array def store", uint64(t.statistics.defaultStore))
	target.UpdateUint64("array def as-array", uint64(t.statistics.defaultAsArray))
	target.UpdateUint64("array sel as-array", uint64(t.statistics.selectAsArray))
	return nil
}

func (t *SMTArrayTheory) ResetStatistics() {
	if t != nil {
		t.statistics = smtArrayStatistics{}
	}
}

func (t *SMTArrayTheory) InitializeModel(generator *SMTModelGenerator) error {
	state, err := t.State()
	if err != nil || generator == nil || generator.manager != state.manager || generator.model == nil {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	factory, err := protomodel.NewArrayFactory(generator.model)
	if err != nil {
		return err
	}
	if err := generator.RegisterFactory(factory); err != nil {
		return err
	}
	return t.prepareArrayModel(generator)
}

type smtArrayTableValueProcedure struct {
	sort, rangeSort, fallback ast.ASTHandle
	domain                    []ast.ASTHandle
	dependencies              []SMTModelValueDependency
	entries                   int
	fallbackValue             bool
	unspecifiedElse           bool
}

func (p *smtArrayTableValueProcedure) Dependencies() []SMTModelValueDependency {
	if p == nil {
		return nil
	}
	return append([]SMTModelValueDependency(nil), p.dependencies...)
}

func (p *smtArrayTableValueProcedure) MakeValue(generator *SMTModelGenerator, values []ast.ASTHandle) (value ast.ASTHandle, err error) {
	if p == nil || generator == nil || generator.model == nil || p.sort.IsNull() || len(p.domain) == 0 || len(values) != len(p.dependencies) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	declaration, err := generator.manager.MkFreshFuncDecl(usymbol.Symbol{}, usymbol.Symbol{}, p.domain, p.rangeSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	interpretation, err := model.NewFunctionInterpretation(generator.manager, uint32(len(p.domain)))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	owned := true
	defer func() {
		if owned {
			err = errors.Join(err, interpretation.Close())
		}
	}()
	position := 0
	fallback := p.fallback
	if p.fallbackValue {
		fallback = values[position]
		position++
	}
	if fallback.IsNull() && !p.unspecifiedElse {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if err = interpretation.SetElse(fallback); err != nil {
		return ast.ASTHandle{}, err
	}
	for entry := 0; entry < p.entries; entry++ {
		arguments := append([]ast.ASTHandle(nil), values[position:position+len(p.domain)]...)
		position += len(p.domain)
		if err = interpretation.InsertEntry(arguments, values[position]); err != nil {
			return ast.ASTHandle{}, err
		}
		position++
	}
	if position != len(values) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	// array_value_proc::mk_value uses register_decl, not register_aux_decl:
	// the as-array value owns a permanent table in this model. Cleanup only
	// removes explicitly registered adjustment auxiliaries.
	if err = generator.model.Core().RegisterFunction(declaration, interpretation); err != nil {
		return ast.ASTHandle{}, err
	}
	owned = false
	return generator.manager.MkAsArray(declaration)
}

func (*smtArrayTableValueProcedure) IsFresh() bool { return false }

func (t *SMTArrayTheory) arrayTableModelValue(node *SMTEnode, generator *SMTModelGenerator, sort ast.ASTHandle) (SMTModelValueProcedure, error) {
	arity, err := generator.manager.ArrayArity(sort)
	if err != nil || arity == 0 {
		if err != nil {
			return nil, err
		}
		return nil, ast.ErrASTDomain
	}
	domain := make([]ast.ASTHandle, arity)
	for index := uint32(0); index < arity; index++ {
		domain[index], err = generator.manager.ArrayDomain(sort, index)
		if err != nil {
			return nil, err
		}
	}
	rangeSort, err := generator.manager.ArrayRange(sort)
	if err != nil {
		return nil, err
	}
	procedure := &smtArrayTableValueProcedure{sort: sort, rangeSort: rangeSort, domain: domain}
	data := t.modelData
	if data == nil {
		return nil, ast.ErrASTDomain
	}
	variable := node.TheoryVar(t.FamilyID())
	if variable < 0 {
		return nil, ast.ErrASTDomain
	}
	root := data.find(int(variable))
	if data.unspecified {
		procedure.unspecifiedElse = true
	} else if fallbackNode := data.defaults[root]; fallbackNode != nil {
		dependency, err := NewSMTEnodeValueDependency(fallbackNode)
		if err != nil {
			return nil, err
		}
		procedure.dependencies = append(procedure.dependencies, dependency)
		procedure.fallbackValue = true
	} else {
		fallback := &data.elseValues[root]
		if fallback.expression.IsNull() && fallback.fresh == nil {
			infinite, err := generator.manager.SortIsInfinite(rangeSort)
			if err != nil {
				return nil, err
			}
			if infinite {
				fallback.fresh, err = generator.MkExtraFreshValue(rangeSort)
			} else {
				fallback.expression, err = generator.SomeValue(rangeSort)
			}
			if err != nil {
				return nil, err
			}
		}
		if fallback.fresh != nil {
			dependency, err := NewSMTFreshValueDependency(fallback.fresh)
			if err != nil {
				return nil, err
			}
			procedure.dependencies = append(procedure.dependencies, dependency)
			procedure.fallbackValue = true
		} else {
			procedure.fallback = fallback.expression
		}
	}
	for _, read := range data.selects[node.root] {
		if len(read.arguments) != int(arity)+1 {
			return nil, ast.ErrASTDomain
		}
		for _, index := range read.arguments[1:] {
			dependency, err := NewSMTEnodeValueDependency(index)
			if err != nil {
				return nil, err
			}
			procedure.dependencies = append(procedure.dependencies, dependency)
		}
		dependency, err := NewSMTEnodeValueDependency(read)
		if err != nil {
			return nil, err
		}
		procedure.dependencies = append(procedure.dependencies, dependency)
		procedure.entries++
	}
	if err := t.traceArrayModelValue(generator, node, data.selects[node.root], data.defaults[root]); err != nil {
		return nil, err
	}
	return procedure, nil
}

func (t *SMTArrayTheory) ModelValue(node *SMTEnode, generator *SMTModelGenerator) (SMTModelValueProcedure, error) {
	if node == nil || generator == nil || !t.IsAttachedToVariable(node) || generator.model == nil {
		return nil, ast.ErrASTDomain
	}
	sort, err := generator.manager.GetSort(node.owner)
	if err != nil {
		return nil, err
	}

	return t.arrayTableModelValue(node, generator, sort)
}
