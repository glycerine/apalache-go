package pdr

// Source: the retained state-symbol, formula-conversion, cube/lemma and SMT
// context operations in src/muz/pdr/pdr_manager.h and pdr_manager.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
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
	manager    *ast.ASTManager
	mux        *SymbolMux
	contexts   *SMTContextManager
	background ast.ASTRef
	oldZero    []ast.ASTHandle
	closed     bool
}

func NewManager(manager *ast.ASTManager, params *uparams.ParamsRef, maximumContexts uint32, config smtcore.SMTSolverConfig) (_ *Manager, resultErr error) {
	if manager == nil || config.Params == nil {
		return nil, ErrManagerOwner
	}
	mux, err := NewSymbolMux(manager)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, mux.Close())
		}
	}()
	contexts, err := NewSMTContextManager(manager, params, maximumContexts, config)
	if err != nil {
		return nil, err
	}
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, contexts.Close())
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
	result := &Manager{manager: manager, mux: mux, contexts: contexts, background: background}
	committed = true
	return result, nil
}

func (m *Manager) open() error {
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
	return m.manager.MkAndArray(expressions)
}

func (m *Manager) CoreIntoCube(expressions []ast.ASTHandle) (ast.ASTHandle, error) {
	return m.ModelIntoCube(expressions)
}

func (m *Manager) CubeIntoLemma(cube ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.manager.MkNotSimplified(cube)
}

func (m *Manager) LemmaIntoCube(lemma ast.ASTHandle) (ast.ASTHandle, error) {
	return m.CubeIntoLemma(lemma)
}

func (m *Manager) NotAnd(expressions []ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	flat, err := m.manager.FlattenAnd(append([]ast.ASTHandle(nil), expressions...))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for index := range flat {
		flat[index], err = m.manager.MkNotSimplified(flat[index])
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return m.manager.MkOrArray(flat)
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
	if m == nil || m.manager == nil {
		return ErrManagerOwner
	}
	if m.closed {
		return nil
	}
	result = errors.Join(result, m.contexts.Close())
	if result != nil {
		return result
	}
	result = errors.Join(result, m.mux.Close())
	result = errors.Join(result, m.manager.DelRef(&m.background))
	if result == nil {
		m.manager = nil
		m.mux = nil
		m.contexts = nil
		m.oldZero = nil
		m.closed = true
	}
	return result
}
