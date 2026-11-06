package pdr

// Source: the retained state-symbol, formula-conversion, cube/lemma and SMT
// context operations in src/muz/pdr/pdr_manager.h and pdr_manager.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
)

var ErrManagerOwner = errors.New("muz/pdr: invalid or closed manager")

// Manager owns the shared PDR state-symbol multiplexer and guarded SMT
// contexts. State declarations are the zero-arity declarations supported by
// the retained native PDR engine.
type Manager struct {
	manager        *ast.ASTManager
	brwr           *rewriter.BoolRewriter
	mux            *SymbolMux
	contexts       *SMTContextManager
	background     ast.ASTRef
	oldZero        []ast.ASTHandle
	notAndPending  []ast.ASTRef
	notAndDeferred []ast.ASTRef
	notAndBorrowed []ast.ASTRef
	notAndActive   bool
	closing        bool
	closed         bool
}

func NewManager(manager *ast.ASTManager, params *uparams.ParamsRef, maximumContexts uint32, config smtcore.SMTSolverConfig) (_ *Manager, resultErr error) {
	if manager == nil || config.Params == nil {
		return nil, ErrManagerOwner
	}
	var brwr *rewriter.BoolRewriter
	var err error
	if config.Globals != nil {
		brwr, err = rewriter.NewBoolRewriter(manager, config.Params, config.Globals, nil)
		if err != nil {
			if brwr != nil {
				err = errors.Join(err, brwr.Close())
			}
			return nil, err
		}
	}
	committed := false
	defer func() {
		if !committed && brwr != nil {
			resultErr = errors.Join(resultErr, brwr.Close())
		}
	}()
	mux, err := NewSymbolMux(manager)
	if err != nil {
		return nil, err
	}
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, mux.Close())
		}
	}()
	truth, err := manager.MkTrue()
	if err != nil {
		return nil, err
	}
	background, err := manager.NewRef(truth)
	if err != nil {
		return nil, err
	}
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, manager.DelRef(&background))
		}
	}()
	contexts, err := NewSMTContextManager(manager, params, maximumContexts, config)
	if err != nil {
		return nil, err
	}
	result := &Manager{manager: manager, brwr: brwr, mux: mux, contexts: contexts, background: background}
	committed = true
	return result, nil
}

func (m *Manager) open() error {
	if m != nil && m.closing && !m.closed {
		return ast.ErrASTPending
	}
	if m == nil || m.closed || m.manager == nil || m.mux == nil || m.contexts == nil {
		return ErrManagerOwner
	}
	_, err := m.manager.ProofsEnabled()
	return err
}

func (m *Manager) Background() (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.manager.RefGet(m.background)
}

func (m *Manager) SetBackground(background ast.ASTHandle) error {
	if err := m.open(); err != nil {
		return err
	}
	return m.manager.SetRef(m.background, background)
}

func (m *Manager) NewPropSolver(name string) (*PropSolver, error) {
	background, err := m.Background()
	if err != nil {
		return nil, err
	}
	return NewPropSolver(m.manager, m.contexts, background, name)
}

func (m *Manager) NewReachableCache(mode ReachableCacheMode) (*ReachableCache, error) {
	background, err := m.Background()
	if err != nil {
		return nil, err
	}
	return NewReachableCache(m.manager, m.contexts, background, mode)
}

func oldIndex(index uint32) uint32 { return index + 1 }

func (m *Manager) addState(prefix ast.ASTHandle) error {
	domain, rangeSort, err := m.mux.declarationSignature(prefix)
	if err != nil {
		return err
	}
	if len(domain) != 0 {
		return fmt.Errorf("%w: PDR state declarations must have arity zero", ast.ErrASTDomain)
	}
	tuple, err := m.mux.CreateTuple(prefix, domain, rangeSort, 2, nil)
	if err != nil {
		return err
	}
	m.oldZero = append(m.oldZero, tuple[oldIndex(0)])
	return nil
}

func (m *Manager) statePredicate(prefix ast.ASTHandle, index uint32) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	result, found, err := m.mux.TryByPrefix(prefix, index)
	if err != nil || found {
		return result, err
	}
	if err = m.addState(prefix); err != nil {
		return ast.ASTHandle{}, err
	}
	result, found, err = m.mux.TryByPrefix(prefix, index)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !found {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return result, nil
}

func (m *Manager) NewPredicate(prefix ast.ASTHandle) (ast.ASTHandle, error) {
	return m.statePredicate(prefix, 0)
}

func (m *Manager) OldPredicate(prefix ast.ASTHandle, index uint32) (ast.ASTHandle, error) {
	return m.statePredicate(prefix, oldIndex(index))
}

func (m *Manager) OldZeroPredicates() ([]ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), m.oldZero...), nil
}

func (m *Manager) IsStatePredicate(declaration ast.ASTHandle) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	return m.mux.IsMuxed(declaration)
}

func (m *Manager) IsOldPredicate(declaration ast.ASTHandle, index uint32) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	return m.mux.HasIndex(declaration, oldIndex(index))
}

func (m *Manager) IsNewPredicate(declaration ast.ASTHandle) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	return m.mux.HasIndex(declaration, 0)
}

func (m *Manager) OldToNew(declaration ast.ASTHandle, old uint32) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.mux.Convert(declaration, oldIndex(old), 0)
}

func (m *Manager) NewToOld(declaration ast.ASTHandle, old uint32) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.mux.Convert(declaration, 0, oldIndex(old))
}

func (m *Manager) OldToOld(declaration ast.ASTHandle, source, target uint32) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.mux.Convert(declaration, oldIndex(source), oldIndex(target))
}

func (m *Manager) FormulaOldToNew(formula ast.ASTHandle, old uint32, homogeneous bool) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.mux.ConvertFormula(formula, oldIndex(old), 0, homogeneous)
}

func (m *Manager) FormulaNewToOld(formula ast.ASTHandle, old uint32, homogeneous bool) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.mux.ConvertFormula(formula, 0, oldIndex(old), homogeneous)
}

// FormulaNewToOldRef returns a caller-owned converted formula root for
// short-lived export paths that must not extend the multiplexer lifetime.
func (m *Manager) FormulaNewToOldRef(formula ast.ASTHandle, old uint32, homogeneous bool) (ast.ASTRef, error) {
	if err := m.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return m.mux.convertFormulaRef(formula, 0, oldIndex(old), homogeneous)
}

func (m *Manager) FormulaOldToOld(formula ast.ASTHandle, source, target uint32, homogeneous bool) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.mux.ConvertFormula(formula, oldIndex(source), oldIndex(target), homogeneous)
}

func (m *Manager) IsOldFormula(formula ast.ASTHandle) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	contains, err := m.mux.Contains(formula, 0)
	return !contains, err
}

func (m *Manager) IsOldFormulaAt(formula ast.ASTHandle, old uint32) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	return m.mux.IsHomogeneousAt(formula, oldIndex(old))
}

func (m *Manager) IsNewFormula(formula ast.ASTHandle) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	return m.mux.IsHomogeneousAt(formula, 0)
}

func (m *Manager) IsHomogeneousFormula(formula ast.ASTHandle) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	return m.mux.IsHomogeneous(formula)
}

func (m *Manager) MarkNonModel(declaration ast.ASTHandle) error {
	if err := m.open(); err != nil {
		return err
	}
	return m.mux.MarkNonModel(declaration)
}

func (m *Manager) ModelIntoCube(expressions []ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := m.AndRef(expressions)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	m.notAndBorrowed = append(m.notAndBorrowed, result)
	return m.manager.RefGet(result)
}

func (m *Manager) CoreIntoCube(expressions []ast.ASTHandle) (ast.ASTHandle, error) {
	return m.ModelIntoCube(expressions)
}

func (m *Manager) CubeIntoLemma(cube ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.booleanNot(cube)
}

func (m *Manager) LemmaIntoCube(lemma ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.booleanNot(lemma)
}

// booleanNot applies the manager's source Boolean rewriter and keeps the
// result alive for the borrowed-handle conversion methods.
func (m *Manager) booleanNot(expression ast.ASTHandle) (ast.ASTHandle, error) {
	result, err := m.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		if result != (ast.ASTRef{}) {
			if cleanup := m.releaseNotAndRef(&result); cleanup != nil {
				err = errors.Join(err, cleanup)
			}
		}
		return ast.ASTHandle{}, err
	}
	if m.brwr != nil {
		err = m.brwr.MkNot(expression, result)
	} else {
		err = rewriter.SimplifyBooleanGateDefault(m.manager, ast.ASTOpNot, []ast.ASTHandle{expression}, result)
	}
	if err != nil {
		return ast.ASTHandle{}, errors.Join(err, m.releaseNotAndRef(&result))
	}
	m.notAndBorrowed = append(m.notAndBorrowed, result)
	return m.manager.RefGet(result)
}

// releaseNotAndPending keeps any reference whose destruction could not finish
// so a later operation or Close can resume it.
func (m *Manager) releaseNotAndPending() error {
	for len(m.notAndPending) != 0 {
		i := len(m.notAndPending) - 1
		err := m.manager.DelRef(&m.notAndPending[i])
		if m.notAndPending[i] == (ast.ASTRef{}) {
			m.notAndPending = m.notAndPending[:i]
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// releaseNotAndRef transfers a lexical result back to the manager before its
// potentially interruptible destruction.
func (m *Manager) releaseNotAndRef(ref *ast.ASTRef) error {
	if m.notAndActive {
		m.notAndDeferred = append(m.notAndDeferred, *ref)
		*ref = ast.ASTRef{}
		return ast.ErrASTPending
	}
	m.notAndPending = append(m.notAndPending, *ref)
	*ref = ast.ASTRef{}
	m.notAndActive = true
	defer m.finishNotAndActive()
	return m.releaseNotAndPending()
}

// finishNotAndActive publishes callback-transferred refs only after the
// current DelRef no longer holds a pointer into the pending slice.
func (m *Manager) finishNotAndActive() {
	m.notAndActive = false
	m.notAndPending = append(m.notAndPending, m.notAndDeferred...)
	m.notAndDeferred = nil
}

// AndRef applies the source manager's Boolean conjunction rewriter and
// returns an owning result reference. The caller releases it after use.
func (m *Manager) AndRef(expressions []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if err := m.open(); err != nil {
		return result, err
	}
	if m.notAndActive {
		return result, ast.ErrASTPending
	}
	m.notAndActive = true
	defer m.finishNotAndActive()
	if err := m.releaseNotAndPending(); err != nil {
		return result, err
	}
	out, err := m.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	m.notAndPending = append(m.notAndPending, out)
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, m.releaseNotAndPending())
		}
	}()
	if m.brwr != nil {
		err = m.brwr.MkAnd(expressions, out)
	} else {
		err = rewriter.SimplifyBooleanGateDefault(m.manager, ast.ASTOpAnd, expressions, out)
	}
	if err != nil {
		return result, err
	}
	result = m.notAndPending[0]
	m.notAndPending = m.notAndPending[:0]
	return result, nil
}

// NotAndRef returns the source bool_rewriter's negated conjunction as an
// owning reference. The caller must release it after the consumer retains it.
func (m *Manager) NotAndRef(expressions []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if err := m.open(); err != nil {
		return result, err
	}
	if m.notAndActive {
		return result, ast.ErrASTPending
	}
	m.notAndActive = true
	defer m.finishNotAndActive()
	if err := m.releaseNotAndPending(); err != nil {
		return result, err
	}
	flat, err := m.manager.FlattenAnd(append([]ast.ASTHandle(nil), expressions...))
	if err != nil {
		return result, err
	}
	// The source flat vector owns each expression. Reserve slot zero for the
	// result so reverse cleanup can release all operands before publishing it.
	out, err := m.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	m.notAndPending = append(m.notAndPending, out)
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, m.releaseNotAndPending())
		}
	}()
	for _, expression := range flat {
		ref, x := m.manager.NewRef(expression)
		if x != nil {
			return result, x
		}
		m.notAndPending = append(m.notAndPending, ref)
	}
	for i := range flat {
		ref := m.notAndPending[i+1]
		if m.brwr != nil {
			err = m.brwr.MkNot(flat[i], ref)
		} else {
			err = rewriter.SimplifyBooleanGateDefault(m.manager, ast.ASTOpNot, flat[i:i+1], ref)
		}
		if err != nil {
			return result, err
		}
		flat[i], err = m.manager.RefGet(ref)
		if err != nil {
			return result, err
		}
	}
	if m.brwr != nil {
		err = m.brwr.MkOr(flat, out)
	} else {
		err = rewriter.SimplifyBooleanGateDefault(m.manager, ast.ASTOpOr, flat, out)
	}
	if err != nil {
		return result, err
	}
	for len(m.notAndPending) > 1 {
		i := len(m.notAndPending) - 1
		err = m.manager.DelRef(&m.notAndPending[i])
		if m.notAndPending[i] == (ast.ASTRef{}) {
			m.notAndPending = m.notAndPending[:i]
		}
		if err != nil {
			return result, err
		}
	}
	result = m.notAndPending[0]
	m.notAndPending = m.notAndPending[:0]
	return result, nil
}

// NotAnd retains its borrowed result for the manager lifetime, as the
// multiplexer does for its borrowed conversion results.
func (m *Manager) NotAnd(expressions []ast.ASTHandle) (ast.ASTHandle, error) {
	ref, err := m.NotAndRef(expressions)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	m.notAndBorrowed = append(m.notAndBorrowed, ref)
	return m.manager.RefGet(ref)
}

func (m *Manager) GetOr(expression ast.ASTHandle) ([]ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	return m.manager.FlattenOr([]ast.ASTHandle{expression})
}

func (m *Manager) matchEquality(expression ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	kind, err := m.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	family, err := m.manager.GetFamilyID(expression)
	if err != nil || family != m.manager.BasicFamilyID() {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	declarationKind, err := m.manager.GetDeclKind(expression)
	if err != nil || declarationKind != ast.ASTOpEq && declarationKind != ast.ASTOpIff {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	left, err := m.manager.GetArg(expression, 0)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	right, err := m.manager.GetArg(expression, 1)
	return left, right, err == nil, err
}

// TryStateAndValueFromAtom recognizes state, (not state), and equalities with
// a state application on either side. Returned handles are borrowed.
func (m *Manager) TryStateAndValueFromAtom(atom ast.ASTHandle) (state, value ast.ASTHandle, found bool, resultErr error) {
	if err := m.open(); err != nil {
		return state, value, false, err
	}
	kind, err := m.manager.GetKind(atom)
	if err != nil || kind != ast.ASTApp {
		return state, value, false, err
	}
	if negated, argument, x := m.manager.IsNot(atom); x != nil {
		return state, value, false, x
	} else if negated {
		if kind, x = m.manager.GetKind(argument); x != nil || kind != ast.ASTApp {
			return state, value, false, x
		}
		state = argument
		value, err = m.manager.MkFalse()
		if err != nil {
			return state, value, false, err
		}
	} else if left, right, equality, x := m.matchEquality(atom); x != nil {
		return state, value, false, x
	} else if equality {
		leftKind, x := m.manager.GetKind(left)
		if x != nil {
			return state, value, false, x
		}
		rightKind, x := m.manager.GetKind(right)
		if x != nil || leftKind != ast.ASTApp || rightKind != ast.ASTApp {
			return state, value, false, x
		}
		leftDecl, x := m.manager.GetDecl(left)
		if x != nil {
			return state, value, false, x
		}
		muxed, x := m.mux.IsMuxed(leftDecl)
		if x != nil {
			return state, value, false, x
		}
		if !muxed {
			left, right = right, left
		}
		state, value = left, right
	} else {
		state = atom
		value, err = m.manager.MkTrue()
		if err != nil {
			return state, value, false, err
		}
	}
	declaration, err := m.manager.GetDecl(state)
	if err != nil {
		return state, value, false, err
	}
	found, err = m.mux.IsMuxed(declaration)
	return state, value, found, err
}

func (m *Manager) TryStateDeclarationFromAtom(atom ast.ASTHandle) (ast.ASTHandle, bool, error) {
	state, _, found, err := m.TryStateAndValueFromAtom(atom)
	if err != nil || !found {
		return ast.ASTHandle{}, false, err
	}
	declaration, err := m.manager.GetDecl(state)
	return declaration, err == nil, err
}

func (m *Manager) ImplicationSurelyHolds(lhs, rhs, background ast.ASTHandle) (result bool, resultErr error) {
	if err := m.open(); err != nil {
		return false, err
	}
	context, err := m.contexts.Fresh()
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, context.Close()) }()
	if !background.IsNull() {
		if err = context.Assert(background); err != nil {
			return false, err
		}
	}
	if err = context.Assert(lhs); err != nil {
		return false, err
	}
	negated, err := m.manager.MkNotSimplified(rhs)
	if err != nil {
		return false, err
	}
	if err = context.Assert(negated); err != nil {
		return false, err
	}
	status, err := context.Check(nil)
	return status == lbool.False, err
}

func (m *Manager) CollectStatistics(destination *statistics.Statistics) error {
	if err := m.open(); err != nil {
		return err
	}
	if destination == nil {
		return ErrManagerOwner
	}
	return m.contexts.CollectStatistics(destination)
}

func (m *Manager) ResetStatistics() error {
	if err := m.open(); err != nil {
		return err
	}
	return m.contexts.ResetStatistics()
}

func (m *Manager) Close() (result error) {
	if m == nil {
		return ErrManagerOwner
	}
	if m.closed {
		return nil
	}
	if m.manager == nil {
		return ErrManagerOwner
	}
	if m.notAndActive {
		return ast.ErrASTPending
	}
	m.closing = true
	m.notAndActive = true
	defer m.finishNotAndActive()
	if result = m.releaseNotAndPending(); result != nil {
		return result
	}
	if len(m.notAndDeferred) != 0 {
		return ast.ErrASTPending
	}
	for len(m.notAndBorrowed) != 0 {
		i := len(m.notAndBorrowed) - 1
		result = m.manager.DelRef(&m.notAndBorrowed[i])
		if m.notAndBorrowed[i] == (ast.ASTRef{}) {
			m.notAndBorrowed = m.notAndBorrowed[:i]
		}
		if result != nil {
			return result
		}
		if len(m.notAndDeferred) != 0 {
			return ast.ErrASTPending
		}
	}
	if result = m.contexts.Close(); result != nil {
		return result
	}
	if len(m.notAndDeferred) != 0 {
		return ast.ErrASTPending
	}
	// Source member destruction is reverse declaration order: contexts, the
	// background expr_ref, sym_mux, then bool_rewriter. Stop at each refused owner so later
	// siblings remain untouched and retry resumes at the same boundary.
	if m.background != (ast.ASTRef{}) {
		if result = m.manager.DelRef(&m.background); result != nil {
			return result
		}
	}
	if len(m.notAndDeferred) != 0 {
		return ast.ErrASTPending
	}
	if result = m.mux.Close(); result != nil {
		return result
	}
	if len(m.notAndDeferred) != 0 {
		return ast.ErrASTPending
	}
	if m.brwr != nil {
		if result = m.brwr.Close(); result != nil {
			return result
		}
	}
	if len(m.notAndDeferred) != 0 {
		return ast.ErrASTPending
	}
	m.brwr = nil
	m.manager = nil
	m.mux = nil
	m.contexts = nil
	m.oldZero = nil
	m.closing = false
	m.closed = true
	return nil
}
