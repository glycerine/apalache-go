package spacer

// Source: the retained state-symbol, background, solver-factory, and
// statistics paths in src/muz/spacer/spacer_manager.h and spacer_manager.cpp.

import (
	"errors"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/cooperate"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
)

var ErrManagerOwner = errors.New("muz/spacer: invalid or closed manager")

type Manager struct {
	manager       *ast.ASTManager
	mux           *SymMux
	background    ast.ASTRef
	contexts      [3]*SMTContextManager
	cooperation   *cooperate.Cooperation
	paramsContext *uparams.ParamsContext
	solverParams  uparams.ParamsRef
	useNativeMBP  bool
	ownsParams    bool
	unique        uint32
	closed        bool
}

func NewManager(manager *ast.ASTManager, params *uparams.ParamsRef, maximumContexts uint32, config smtcore.SMTSolverConfig) (_ *Manager, resultErr error) {
	if manager == nil || config.Params == nil || config.Globals == nil {
		return nil, ErrManagerOwner
	}
	mux, err := NewSymMux(manager, []string{"_n"})
	if err != nil {
		return nil, err
	}
	truth, err := manager.MkTrue()
	if err != nil {
		return nil, errors.Join(err, mux.Close())
	}
	background, err := manager.NewRef(truth)
	if err != nil {
		return nil, errors.Join(err, mux.Close())
	}
	result := &Manager{manager: manager, mux: mux, background: background, paramsContext: config.Params}
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	result.solverParams, err = spacerSolverParams(params, config)
	if err != nil {
		return nil, err
	}
	fixedpoint, err := config.Globals.SourceGetModuleName("fixedpoint")
	if err != nil {
		return nil, err
	}
	nativeMBPName := "spacer.native_mbp"
	result.useNativeMBP, err = config.Params.GetBoolName(&fixedpoint, &nativeMBPName, false)
	if err != nil {
		return nil, err
	}
	result.ownsParams = true
	if config.ModelEvaluator.Context == nil {
		config.ModelEvaluator.Context = config.Params
	}
	if config.ModelEvaluator.Globals == nil {
		config.ModelEvaluator.Globals = config.Globals
	}
	if config.ModelEvaluator.Memory == nil {
		config.ModelEvaluator.Memory = manager.NumeralService().RuntimeOwner()
	}
	if config.ModelEvaluator.Cooperation == nil {
		result.cooperation, err = cooperate.NewCooperation(manager.Budget())
		if err != nil {
			return nil, err
		}
		config.ModelEvaluator.Cooperation = result.cooperation
	}
	for index := range result.contexts {
		result.contexts[index], err = NewSMTContextManager(manager, &result.solverParams, maximumContexts, config)
		if err != nil {
			return nil, err
		}
	}
	committed = true
	return result, nil
}

func spacerSolverParams(source *uparams.ParamsRef, config smtcore.SMTSolverConfig) (result uparams.ParamsRef, resultErr error) {
	if config.Params == nil || config.Globals == nil {
		return uparams.ParamsRef{}, ErrManagerOwner
	}
	fixedpoint, err := config.Globals.SourceGetModuleName("fixedpoint")
	if err != nil {
		return uparams.ParamsRef{}, err
	}
	seedName := "spacer.random_seed"
	seed, err := config.Params.GetUIntName(&fixedpoint, &seedName, 0)
	if err != nil {
		return uparams.ParamsRef{}, err
	}
	mbqiName := "spacer.mbqi"
	mbqi, err := config.Params.GetBoolName(&fixedpoint, &mbqiName, true)
	if err != nil {
		return uparams.ParamsRef{}, err
	}
	equalityName := "spacer.eq_prop"
	equalityPropagation, err := config.Params.GetBoolName(&fixedpoint, &equalityName, true)
	if err != nil {
		return uparams.ParamsRef{}, err
	}
	result = config.Params.EmptyValue()
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, config.Params.Del(&result))
		}
	}()
	randomSeedName := "random_seed"
	if err = config.Params.SetUIntName(&result, &randomSeedName, seed); err != nil {
		return uparams.ParamsRef{}, err
	}
	if source != nil {
		if err = config.Params.Append(&result, source); err != nil {
			return uparams.ParamsRef{}, err
		}
	}
	if err = config.Params.SetUIntName(&result, &randomSeedName, seed); err != nil {
		return uparams.ParamsRef{}, err
	}
	smtMBQIName := "mbqi"
	if err = config.Params.SetBoolName(&result, &smtMBQIName, mbqi); err != nil {
		return uparams.ParamsRef{}, err
	}
	if !equalityPropagation {
		propagationName := "arith.propagation_mode"
		if err = config.Params.SetUIntName(&result, &propagationName, 0); err != nil {
			return uparams.ParamsRef{}, err
		}
		propagateEqualitiesName := "arith.propagate_eqs"
		if err = config.Params.SetBoolName(&result, &propagateEqualitiesName, false); err != nil {
			return uparams.ParamsRef{}, err
		}
	}
	committed = true
	return result, nil
}

func (m *Manager) open() error {
	if m == nil || m.closed || m.manager == nil || m.mux == nil {
		return ErrManagerOwner
	}
	return nil
}

func (m *Manager) Mux() (*SymMux, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	return m.mux, nil
}

func (m *Manager) addState(prefix ast.ASTHandle) error {
	arity, err := m.manager.GetArity(prefix)
	if err != nil {
		return err
	}
	if arity != 0 {
		return ast.ErrASTDomain
	}
	_, err = m.mux.CreateTuple(prefix, 2)
	return err
}

func (m *Manager) NewPredicate(prefix ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	value, found, err := m.mux.ByPrefix(prefix, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !found {
		if err = m.addState(prefix); err != nil {
			return ast.ASTHandle{}, err
		}
		value, found, err = m.mux.ByPrefix(prefix, 0)
	}
	if err != nil || !found {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, err
	}
	return value, nil
}

func (m *Manager) OldPredicate(prefix ast.ASTHandle, oldIndex uint32) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	value, found, err := m.mux.ByPrefix(prefix, oldIndex+1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !found {
		if err = m.addState(prefix); err != nil {
			return ast.ASTHandle{}, err
		}
		value, found, err = m.mux.ByPrefix(prefix, oldIndex+1)
	}
	if err != nil || !found {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, err
	}
	return value, nil
}

func (m *Manager) OldToNew(declaration ast.ASTHandle, oldIndex uint32) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.mux.ConvertDeclaration(declaration, oldIndex+1, 0)
}

func (m *Manager) NewToOld(declaration ast.ASTHandle, oldIndex uint32) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.mux.ConvertDeclaration(declaration, 0, oldIndex+1)
}

func (m *Manager) OldToOld(declaration ast.ASTHandle, source, target uint32) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.mux.ConvertDeclaration(declaration, source+1, target+1)
}

func (m *Manager) FormulaNewToOld(formula ast.ASTHandle, oldIndex uint32, homogeneous bool) (ast.ASTRef, error) {
	if err := m.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return m.mux.ConvertFormula(formula, 0, oldIndex+1, homogeneous)
}

func (m *Manager) FormulaOldToNew(formula ast.ASTHandle, oldIndex uint32, homogeneous bool) (ast.ASTRef, error) {
	if err := m.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return m.mux.ConvertFormula(formula, oldIndex+1, 0, homogeneous)
}

func (m *Manager) IsOldFormula(formula ast.ASTHandle) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	containsNew, err := m.mux.Contains(formula, 0)
	return !containsNew, err
}

func (m *Manager) IsOldFormulaAt(formula ast.ASTHandle, oldIndex uint32) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	return m.mux.IsHomogeneousAt(formula, oldIndex+1)
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

func (m *Manager) CollectStateIndices(formula ast.ASTHandle) ([]uint32, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	return m.mux.CollectIndices(formula)
}

func (m *Manager) CollectStateVariables(formula ast.ASTHandle) ([][]ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	return m.mux.CollectVariables(formula)
}

func (m *Manager) HaveDifferentStateKinds(left, right ast.ASTHandle) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	leftIndex, leftMuxed, err := m.mux.Index(left)
	if err != nil {
		return false, err
	}
	rightIndex, rightMuxed, err := m.mux.Index(right)
	return leftMuxed && rightMuxed && leftIndex != rightIndex, err
}

func (m *Manager) FilterOldAtoms(formulas []ast.ASTHandle, oldIndex uint32) ([]ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	return m.mux.FilterIndex(formulas, oldIndex+1)
}

func (m *Manager) FilterNewAtoms(formulas []ast.ASTHandle) ([]ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	return m.mux.FilterIndex(formulas, 0)
}

func (m *Manager) PartitionOldAtoms(formulas []ast.ASTHandle, oldIndex uint32) ([]ast.ASTHandle, []ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return nil, nil, err
	}
	return m.mux.PartitionIndex(formulas, oldIndex+1)
}

func (m *Manager) FilterNonModelAtoms(formulas []ast.ASTHandle) ([]ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	return m.mux.FilterNonModel(formulas)
}

func (m *Manager) ModelString(modelCore *model.ModelCore) (string, error) {
	if err := m.open(); err != nil {
		return "", err
	}
	return m.mux.ModelString(modelCore)
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
	operation, err := m.manager.GetDeclKind(expression)
	if err != nil || operation != ast.ASTOpEq && operation != ast.ASTOpIff {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	left, err := m.manager.GetArg(expression, 0)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	right, err := m.manager.GetArg(expression, 1)
	return left, right, err == nil, err
}

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
		leftDeclaration, x := m.manager.GetDecl(left)
		if x != nil {
			return state, value, false, x
		}
		muxed, x := m.mux.IsMuxed(leftDeclaration)
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

func MkZKConst(manager *ast.ASTManager, index uint32, sort ast.ASTHandle) (ast.ASTHandle, error) {
	if manager == nil {
		return ast.ASTHandle{}, ErrManagerOwner
	}
	name, err := manager.Symbols.Intern("sk!" + strconv.FormatUint(uint64(index), 10))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkConst(name, sort)
}

func FindZKConstants(manager *ast.ASTManager, expression ast.ASTHandle) ([]ast.ASTHandle, error) {
	if manager == nil {
		return nil, ErrManagerOwner
	}
	var result []ast.ASTHandle
	seen := make(map[ast.ASTHandle]bool)
	stack := []ast.ASTHandle{expression}
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if seen[current] {
			continue
		}
		seen[current] = true
		kind, err := manager.GetKind(current)
		if err != nil {
			return nil, err
		}
		switch kind {
		case ast.ASTApp:
			declaration, err := manager.GetDecl(current)
			if err != nil {
				return nil, err
			}
			family, err := manager.GetFamilyID(declaration)
			if err != nil {
				return nil, err
			}
			arity, err := manager.GetArity(declaration)
			if err != nil {
				return nil, err
			}
			name, err := manager.GetASTName(declaration)
			if err != nil {
				return nil, err
			}
			text, err := manager.Symbols.Display(name)
			if err != nil {
				return nil, err
			}
			if family == ast.NullFamilyID && arity == 0 && strings.HasPrefix(text, "sk!") {
				result = append(result, current)
			}
			count, err := manager.GetNumArgs(current)
			if err != nil {
				return nil, err
			}
			for index := uint32(0); index < count; index++ {
				argument, err := manager.GetArg(current, index)
				if err != nil {
					return nil, err
				}
				stack = append(stack, argument)
			}
		case ast.ASTQuantifier:
			body, err := manager.GetQuantifierBody(current)
			if err != nil {
				return nil, err
			}
			stack = append(stack, body)
		}
	}
	return result, nil
}

func HasZKConst(manager *ast.ASTManager, expression ast.ASTHandle) (bool, error) {
	values, err := FindZKConstants(manager, expression)
	return len(values) != 0, err
}

func (m *Manager) SetBackground(background ast.ASTHandle) error {
	if err := m.open(); err != nil {
		return err
	}
	return m.manager.SetRef(m.background, background)
}

func (m *Manager) Background() (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.manager.RefGet(m.background)
}

func (m *Manager) Unique() (uint32, error) {
	if err := m.open(); err != nil {
		return 0, err
	}
	value := m.unique
	m.unique++
	return value, nil
}

func (m *Manager) Fresh(pool uint32) (*VirtualSolver, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if pool >= uint32(len(m.contexts)) {
		return nil, ast.ErrASTDomain
	}
	return m.contexts[pool].Fresh()
}

func (m *Manager) NewPropSolver(name string, config PropSolverConfig) (_ *PropSolver, resultErr error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	var concrete [2]*VirtualSolver
	var backends [2]solver.Solver
	for index := range concrete {
		created, err := m.contexts[index].Fresh()
		if err != nil {
			for prior := index - 1; prior >= 0; prior-- {
				resultErr = errors.Join(resultErr, concrete[prior].Close())
			}
			return nil, errors.Join(err, resultErr)
		}
		concrete[index], backends[index] = created, created
	}
	background, err := m.Background()
	if err != nil {
		return nil, errors.Join(err, concrete[1].Close(), concrete[0].Close())
	}
	result, err := NewPropSolver(m.manager, backends, background, name, config)
	if err != nil {
		return nil, errors.Join(err, concrete[1].Close(), concrete[0].Close())
	}
	result.ownedBackends = concrete
	return result, nil
}

func (m *Manager) CollectStatistics(destination *statistics.Statistics) error {
	if err := m.open(); err != nil {
		return err
	}
	for _, context := range m.contexts {
		if err := context.CollectStatistics(destination); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) ResetStatistics() error {
	if err := m.open(); err != nil {
		return err
	}
	for _, context := range m.contexts {
		if err := context.ResetStatistics(); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) Close() (result error) {
	if m == nil || m.manager == nil {
		return ErrManagerOwner
	}
	if m.closed {
		return nil
	}
	for _, context := range m.contexts {
		if context == nil {
			continue
		}
		for _, factory := range context.factories {
			for _, virtual := range factory.solvers {
				if virtual != nil && !virtual.closed {
					return ErrManagerOwner
				}
			}
		}
	}
	for index := len(m.contexts) - 1; index >= 0; index-- {
		if m.contexts[index] != nil {
			result = errors.Join(result, m.contexts[index].Close())
		}
	}
	if result == nil && m.ownsParams {
		paramErr := m.paramsContext.Del(&m.solverParams)
		result = errors.Join(result, paramErr)
		if paramErr == nil {
			m.ownsParams = false
		}
	}
	result = errors.Join(result, m.mux.Close(), m.manager.DelRef(&m.background))
	if m.cooperation != nil {
		result = errors.Join(result, m.cooperation.Close())
	}
	if result == nil {
		m.contexts = [3]*SMTContextManager{}
		m.mux = nil
		m.cooperation = nil
		m.paramsContext = nil
		m.manager = nil
		m.closed = true
	}
	return result
}
