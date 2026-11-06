package api

// Source: src/api/api_model.cpp:33-425; src/api/api_model.h:25-56.
// Model printing at api_model.cpp:490-511 is intentionally kept separate.
//
// APIModel and its interpretation wrappers translate the source opaque API
// objects into explicit Go owners. Every wrapper owns a ModelCore reference;
// borrowed FunctionInterpretation and FunctionEntry pointers therefore cannot
// be retired while their API wrapper remains open.

import (
	"bytes"
	"errors"
	"strings"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

type APIModel struct {
	owner  *smtContextState
	core   *model.ModelCore
	closed bool
}

func newAPIModel(owner *smtContextState, core *model.ModelCore, acquired bool) (*APIModel, error) {
	if owner == nil || owner.closed || core == nil {
		return nil, ErrSMTContextOwner
	}
	if !acquired {
		if err := core.IncRef(); err != nil {
			return nil, err
		}
	}
	return &APIModel{owner: owner, core: core}, nil
}

func (c *SMTContext) NewAPIModel() (*APIModel, error) {
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	derived, err := model.NewModel(manager)
	if err != nil {
		return nil, err
	}
	result, err := newAPIModel(c.smtContextState, derived.Core(), false)
	if err != nil {
		return nil, errors.Join(err, derived.Close())
	}
	return result, nil
}

// CurrentAPIModel acquires the latest solver model. It remains valid across
// subsequent checks and scope changes until Close, matching Z3_model_inc_ref.
func (c *SMTContext) CurrentAPIModel() (*APIModel, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	core, err := c.command.Model()
	if err != nil {
		return nil, err
	}
	return newAPIModel(c.smtContextState, core, true)
}

func (m *APIModel) open() error {
	if m == nil || m.owner == nil || m.core == nil || m.closed || m.owner.closed {
		return ErrSMTContextOwner
	}
	return m.core.Check()
}

func (m *APIModel) Manager() (*ast.ASTManager, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	return m.core.Manager(), nil
}

func (m *APIModel) Close() error {
	if m == nil || m.closed {
		return nil
	}
	if m.core == nil {
		return ErrSMTContextOwner
	}
	err := m.core.DecRef()
	if err == nil {
		m.closed = true
		m.owner, m.core = nil, nil
	}
	return err
}

func (m *APIModel) ConstantInterpretation(declaration ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.core.ConstantInterpretation(declaration)
}

func (m *APIModel) HasInterpretation(declaration ast.ASTHandle) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	return m.core.HasInterpretation(declaration)
}

func (m *APIModel) NumConstants() (uint32, error) {
	if err := m.open(); err != nil {
		return 0, err
	}
	return m.core.NumConstants()
}

func (m *APIModel) ConstantDeclaration(index uint32) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.core.Constant(index)
}

func (m *APIModel) NumFunctions() (uint32, error) {
	if err := m.open(); err != nil {
		return 0, err
	}
	return m.core.NumFunctionDeclarations(), nil
}

func (m *APIModel) FunctionDeclaration(index uint32) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.core.Function(index)
}

func (m *APIModel) NumSorts() (uint32, error) {
	if err := m.open(); err != nil {
		return 0, err
	}
	derived := m.core.DerivedModel()
	if derived == nil {
		return 0, ast.ErrASTDomain
	}
	return derived.NumUninterpretedSorts()
}

func (m *APIModel) Sort(index uint32) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	derived := m.core.DerivedModel()
	if derived == nil {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return derived.UninterpretedSort(index)
}

func (m *APIModel) RegisterConstant(declaration, value ast.ASTHandle) error {
	if err := m.open(); err != nil {
		return err
	}
	return m.core.RegisterConstant(declaration, value)
}

func (m *APIModel) RegisterUniverse(sort ast.ASTHandle, values []ast.ASTHandle) error {
	if err := m.open(); err != nil {
		return err
	}
	derived := m.core.DerivedModel()
	if derived == nil {
		return ast.ErrASTDomain
	}
	return derived.RegisterUniverse(sort, values)
}

// APIModelUniverse is the source Z3_ast_vector result: it owns one reference
// per value independently of the model that supplied the universe.
type APIModelUniverse struct {
	manager *ast.ASTManager
	values  []ast.ASTRef
	closed  bool
}

func (m *APIModel) Universe(sort ast.ASTHandle) (result *APIModelUniverse, err error) {
	if err = m.open(); err != nil {
		return nil, err
	}
	derived := m.core.DerivedModel()
	if derived == nil {
		return nil, ast.ErrASTDomain
	}
	values, err := derived.Universe(sort)
	if err != nil {
		return nil, err
	}
	result = &APIModelUniverse{manager: m.core.Manager(), values: make([]ast.ASTRef, 0, len(values))}
	defer func() {
		if err != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	for _, value := range values {
		ref, x := result.manager.NewRef(value)
		if x != nil {
			return nil, x
		}
		result.values = append(result.values, ref)
	}
	return result, nil
}

func (u *APIModelUniverse) Size() uint32 {
	if u == nil || u.closed {
		return 0
	}
	return uint32(len(u.values))
}

func (u *APIModelUniverse) At(index uint32) (ast.ASTHandle, error) {
	if u == nil || u.manager == nil || u.closed {
		return ast.ASTHandle{}, ErrSMTContextOwner
	}
	if uint64(index) >= uint64(len(u.values)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return u.manager.RefGet(u.values[index])
}

func (u *APIModelUniverse) Close() (result error) {
	if u == nil || u.closed {
		return nil
	}
	for i := range u.values {
		result = errors.Join(result, u.manager.DelRef(&u.values[i]))
	}
	if result == nil {
		u.closed = true
		u.manager, u.values = nil, nil
	}
	return result
}

func (m *APIModel) Translate(target *SMTContext) (result *APIModel, err error) {
	if err = m.open(); err != nil {
		return nil, err
	}
	targetManager, err := target.Manager()
	if err != nil {
		return nil, err
	}
	derived := m.core.DerivedModel()
	if derived == nil || target.smtContextState == m.owner {
		return nil, ast.ErrASTDomain
	}
	translation, err := ast.NewASTTranslation(m.core.Manager(), targetManager, true)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, translation.Close()) }()
	translated, err := derived.Translate(translation)
	if err != nil {
		return nil, err
	}
	result, err = newAPIModel(target.smtContextState, translated.Core(), false)
	if err != nil {
		return nil, errors.Join(err, translated.Close())
	}
	return result, nil
}

// String implements Z3_model_to_string using the owning context's live print
// mode and parameter services. SMT2 mode removes the printer's trailing line
// break exactly as the source API shim does.
func (m *APIModel) String() (text string, resultErr error) {
	if err := m.open(); err != nil {
		return "", err
	}
	globals, err := m.owner.command.GlobalParameters()
	if err != nil {
		return "", err
	}
	compliant, err := m.owner.command.Compliant()
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if compliant {
		if err = m.core.WriteSMT2(&out, globals.Context, globals, 0); err != nil {
			return "", err
		}
		return strings.TrimSuffix(out.String(), "\n"), nil
	}
	parameters, err := model.NewModelParams(globals.Context, globals, nil)
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, parameters.Close()) }()
	partial, err := parameters.Partial()
	if err != nil {
		return "", err
	}
	if err = m.core.WriteV2(&out, globals.Context, globals, partial); err != nil {
		return "", err
	}
	return out.String(), nil
}

func (c *SMTContext) IsAsArray(value ast.ASTHandle) (bool, error) {
	manager, err := c.Manager()
	if err != nil {
		return false, err
	}
	_, err = manager.AsArrayFunction(value)
	if errors.Is(err, ast.ErrASTDomain) {
		return false, nil
	}
	return err == nil, err
}

func (c *SMTContext) AsArrayFunction(value ast.ASTHandle) (ast.ASTHandle, error) {
	manager, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.AsArrayFunction(value)
}

type APIFunctionInterpretation struct {
	model    *model.ModelCore
	function *model.FunctionInterpretation
	closed   bool
}

func newAPIFunctionInterpretation(core *model.ModelCore, function *model.FunctionInterpretation) (*APIFunctionInterpretation, error) {
	if core == nil || function == nil {
		return nil, ast.ErrASTDomain
	}
	if err := core.IncRef(); err != nil {
		return nil, err
	}
	return &APIFunctionInterpretation{model: core, function: function}, nil
}

func (m *APIModel) FunctionInterpretation(declaration ast.ASTHandle) (*APIFunctionInterpretation, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	function, err := m.core.FunctionInterpretation(declaration)
	if err != nil {
		return nil, err
	}
	return newAPIFunctionInterpretation(m.core, function)
}

func (m *APIModel) AddFunctionInterpretation(declaration, elseValue ast.ASTHandle) (*APIFunctionInterpretation, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	arity, err := m.core.Manager().GetArity(declaration)
	if err != nil {
		return nil, err
	}
	function, err := model.NewFunctionInterpretation(m.core.Manager(), arity)
	if err != nil {
		return nil, err
	}
	if err = function.SetElse(elseValue); err != nil {
		return nil, errors.Join(err, function.Close())
	}
	if err = m.core.RegisterFunction(declaration, function); err != nil {
		return nil, errors.Join(err, function.Close())
	}
	return newAPIFunctionInterpretation(m.core, function)
}

func (f *APIFunctionInterpretation) open() error {
	if f == nil || f.model == nil || f.function == nil || f.closed {
		return ErrSMTContextOwner
	}
	return f.model.Check()
}

func (f *APIFunctionInterpretation) Close() error {
	if f == nil || f.closed {
		return nil
	}
	err := f.model.DecRef()
	if err == nil {
		f.closed = true
		f.model, f.function = nil, nil
	}
	return err
}

func (f *APIFunctionInterpretation) NumEntries() (uint32, error) {
	if err := f.open(); err != nil {
		return 0, err
	}
	return f.function.NumEntries(), nil
}

func (f *APIFunctionInterpretation) Arity() (uint32, error) {
	if err := f.open(); err != nil {
		return 0, err
	}
	return f.function.Arity(), nil
}

func (f *APIFunctionInterpretation) Else() (ast.ASTHandle, error) {
	if err := f.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return f.function.Else()
}

func (f *APIFunctionInterpretation) SetElse(value ast.ASTHandle) error {
	if err := f.open(); err != nil {
		return err
	}
	return f.function.SetElse(value)
}

func (f *APIFunctionInterpretation) AddEntry(arguments []ast.ASTHandle, value ast.ASTHandle) error {
	if err := f.open(); err != nil {
		return err
	}
	if uint64(len(arguments)) != uint64(f.function.Arity()) {
		return arena.ErrBounds
	}
	return f.function.InsertEntry(arguments, value)
}

type APIFunctionEntry struct {
	model    *model.ModelCore
	function *model.FunctionInterpretation
	entry    *model.FunctionEntry
	closed   bool
}

func (f *APIFunctionInterpretation) Entry(index uint32) (*APIFunctionEntry, error) {
	if err := f.open(); err != nil {
		return nil, err
	}
	entry, err := f.function.Entry(index)
	if err != nil {
		return nil, err
	}
	if err = f.model.IncRef(); err != nil {
		return nil, err
	}
	return &APIFunctionEntry{model: f.model, function: f.function, entry: entry}, nil
}

func (e *APIFunctionEntry) open() error {
	if e == nil || e.model == nil || e.function == nil || e.entry == nil || e.closed {
		return ErrSMTContextOwner
	}
	return e.model.Check()
}

func (e *APIFunctionEntry) Close() error {
	if e == nil || e.closed {
		return nil
	}
	err := e.model.DecRef()
	if err == nil {
		e.closed = true
		e.model, e.function, e.entry = nil, nil, nil
	}
	return err
}

func (e *APIFunctionEntry) Value() (ast.ASTHandle, error) {
	if err := e.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return e.entry.Result()
}

func (e *APIFunctionEntry) NumArguments() (uint32, error) {
	if err := e.open(); err != nil {
		return 0, err
	}
	return e.function.Arity(), nil
}

func (e *APIFunctionEntry) Argument(index uint32) (ast.ASTHandle, error) {
	if err := e.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return e.entry.Argument(index)
}
