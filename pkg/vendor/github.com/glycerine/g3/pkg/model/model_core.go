package model

// Source: src/model/model_core.h:31-78; src/model/model_core.cpp:21-66,85-109;
// src/model/model.cpp:44-50.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/insdict"
)

// Original constant/function interpretation ownership. The derived model
// supplies universe lifetimes and virtual final-reference destruction.
type ModelCore struct {
	self                               *ModelCore
	manager                            *ast.ASTManager
	Constants                          insdict.Dict[ast.ASTHandle, *modelConstantEntry]
	Functions                          insdict.Dict[ast.ASTHandle, *modelFunctionEntry]
	removedFunction                    *modelFunctionEntry
	removedConstant                    *modelConstantEntry
	copiedFunction                     *FunctionInterpretation
	functionDeclarations               []ast.ASTHandle
	Derived                            *Model
	valueProvider                      ModelValueProvider
	declarations, ConstantDeclarations []ast.ASTHandle
	References                         uint32
	ownerBytes                         uint64
	Closed                             bool
}

// ModelValueProvider is the virtual get_some_value surface used by model
// construction. Proto-models replace the final model's universe provider while
// they own the core.
type ModelValueProvider interface {
	GetSomeValue(ast.ASTHandle) (ast.ASTHandle, error)
}

func NewModelCore(m *ast.ASTManager) (md *ModelCore, err error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err = m.ProofsEnabled(); err != nil {
		return nil, err
	}
	amount := uint64(reflect.TypeOf(ModelCore{}).Size())
	if err = m.Budget().Reserve(amount); err != nil {
		return nil, err
	}
	md = &ModelCore{manager: m, ownerBytes: amount}
	md.self = md
	md.Constants = *insdict.NewDictFunc[ast.ASTHandle, *modelConstantEntry](modelHandleHash)
	md.Functions = *insdict.NewDictFunc[ast.ASTHandle, *modelFunctionEntry](modelHandleHash)
	return md, nil
}
func (md *ModelCore) open() error {
	if md == nil || md.self != md {
		return ast.ErrASTHandle
	}
	if md.Closed {
		return arena.ErrClosed
	}
	_, e := md.manager.ProofsEnabled()
	return e
}
func (md *ModelCore) checkDeclaration(d ast.ASTHandle) error {
	if e := md.open(); e != nil {
		return e
	}
	kind, e := md.manager.GetKind(d)
	if e != nil {
		return e
	}
	if kind != ast.ASTFuncDecl {
		return ast.ErrASTDomain
	}
	return nil
}
func (md *ModelCore) RegisterConstant(d, v ast.ASTHandle) error {
	e := md.checkDeclaration(d)
	if e != nil {
		return e
	}
	arity, e := md.manager.GetArity(d)
	if e != nil {
		return e
	}
	if arity != 0 {
		return ast.ErrASTDomain
	}
	kind, e := md.manager.GetKind(v)
	if e != nil {
		return e
	}
	if kind != ast.ASTApp && kind != ast.ASTVar && kind != ast.ASTQuantifier {
		return ast.ErrASTDomain
	}
	entry, found := md.Constants.Get2(d)
	if !found {
		entry = &modelConstantEntry{declaration: d}
		md.Constants.Set(d, entry)
	}
	if !entry.pending.IsNull() {
		return ast.ErrASTPending
	}
	m := md.manager
	if entry.value.IsNull() {
		md.declarations = append(md.declarations, d)
		md.ConstantDeclarations = append(md.ConstantDeclarations, d)
		if e = m.IncRef(d); e != nil {
			return e
		}
		entry.declarationOwned = true
		if e = m.IncRef(v); e != nil {
			return e
		}
		entry.value = v
		entry.valueOwned = true
	} else {
		if e = m.IncRef(v); e != nil {
			return e
		}
		consumed, e := m.DecRefConsumed(entry.value)
		if consumed {
			entry.value = v
			return e
		}
		if e == nil {
			e = ast.ErrASTPending
		}
		entry.pending = v
		consumed, cleanup := m.DecRefConsumed(v)
		if consumed {
			entry.pending = ast.ASTHandle{}
		}
		return errors.Join(e, cleanup)
	}
	return nil
}
func (md *ModelCore) ConstantInterpretation(d ast.ASTHandle) (ast.ASTHandle, error) {
	e := md.checkDeclaration(d)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	entry, found := md.Constants.Get2(d)
	if !found {
		return ast.ASTHandle{}, nil
	}
	return entry.value, nil
}
func (md *ModelCore) HasConstantInterpretation(d ast.ASTHandle) (bool, error) {
	e := md.checkDeclaration(d)
	if e != nil {
		return false, e
	}
	_, found := md.Constants.Get2(d)
	return found, nil
}

// Source model_core::eval's constant branch assigns r even when the declaration
// is absent. The owning ASTRef adapter preserves both assignment and lifetime.
func (md *ModelCore) EvalConstant(d ast.ASTHandle, out ast.ASTRef) (bool, error) {
	if e := md.open(); e != nil {
		return false, e
	}
	arity, e := md.manager.GetArity(d)
	if e != nil {
		return false, e
	}
	if arity != 0 {
		return false, ast.ErrASTDomain
	}
	value, e := md.ConstantInterpretation(d)
	if e != nil {
		return false, e
	}
	if e = md.manager.SetRef(out, value); e != nil {
		return false, e
	}
	return !value.IsNull(), nil
}
func (md *ModelCore) NumDeclarations() (uint32, error) {
	if e := md.open(); e != nil {
		return 0, e
	}
	return uint32(len(md.declarations)), nil
}
func (md *ModelCore) NumConstants() (uint32, error) {
	if e := md.open(); e != nil {
		return 0, e
	}
	return uint32(len(md.ConstantDeclarations)), nil
}
func (md *ModelCore) Declaration(i uint32) (ast.ASTHandle, error) {
	if e := md.open(); e != nil {
		return ast.ASTHandle{}, e
	}
	if uint64(i) >= uint64(len(md.declarations)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return md.declarations[i], nil
}
func (md *ModelCore) Constant(i uint32) (ast.ASTHandle, error) {
	if e := md.open(); e != nil {
		return ast.ASTHandle{}, e
	}
	if uint64(i) >= uint64(len(md.ConstantDeclarations)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return md.ConstantDeclarations[i], nil
}
func (md *ModelCore) UnregisterConstant(d ast.ASTHandle) error {
	e := md.checkDeclaration(d)
	if e != nil {
		return e
	}
	if md.removedConstant != nil {
		return arena.ErrClosed
	}
	entry, found := md.Constants.Get2(d)
	if !found || entry.value.IsNull() {
		return nil
	}
	md.removedConstant = entry
	md.Constants.Del(d)
	for i, h := range md.ConstantDeclarations {
		if h == d {
			copy(md.ConstantDeclarations[i:], md.ConstantDeclarations[i+1:])
			md.ConstantDeclarations[len(md.ConstantDeclarations)-1] = ast.ASTHandle{}
			md.ConstantDeclarations = md.ConstantDeclarations[:len(md.ConstantDeclarations)-1]
			break
		}
	}
	return md.finishConstantRemoval()
}
func (md *ModelCore) CopyConstantInterpretations(source *ModelCore) error {
	if e := md.open(); e != nil {
		return e
	}
	if e := source.open(); e != nil {
		return e
	}
	if md.manager != source.manager {
		return ast.ErrASTHandle
	}
	for d, entry := range source.Constants.All() {
		if e := md.RegisterConstant(d, entry.value); e != nil {
			return e
		}
	}
	return nil
}
func (md *ModelCore) IncRef() error {
	if e := md.open(); e != nil {
		return e
	}
	md.References++
	return nil
}
func (md *ModelCore) DecRef() error {
	_, e := md.DecRefConsumed()
	return e
}

// DecRefConsumed distinguishes a failed precondition from a deletion that
// stopped after consuming this reference. Owning adapters use the receipt to
// avoid decrementing the same model twice during explicit cleanup.
func (md *ModelCore) DecRefConsumed() (bool, error) {
	if e := md.open(); e != nil {
		return false, e
	}
	md.References--
	if md.References == 0 {
		return true, md.Close()
	}
	return true, nil
}
func (md *ModelCore) Close() error {
	if md == nil || md.self != md {
		return ast.ErrASTHandle
	}
	if md.Derived != nil {
		return md.Derived.Close()
	}
	return md.closeCoreMembers()
}
func (md *ModelCore) closeCoreMembers() error {
	if md == nil || md.self != md {
		return ast.ErrASTHandle
	}
	if md.Closed {
		return nil
	}
	m := md.manager
	// Retire constants before functions, in dictionary insertion order.
	for _, entry := range md.Constants.All() {
		if e := md.retireConstant(entry); e != nil {
			return e
		}
	}
	for _, entry := range md.Functions.All() {
		if e := md.retireFunction(entry, 29); e != nil {
			return e
		}
	}
	// A copy stopped before publication retains its lexical function owner.
	if md.copiedFunction != nil {
		if e := md.copiedFunction.Close(); e != nil {
			return e
		}
		md.copiedFunction = nil
	}
	// A stopped unregister has separately retained the removed lexical owners.
	if e := md.finishConstantRemoval(); e != nil {
		return e
	}
	if e := md.finishFunctionRemoval(); e != nil {
		return e
	}
	md.functionDeclarations = nil
	md.ConstantDeclarations = nil
	md.declarations = nil
	md.Functions = insdict.Dict[ast.ASTHandle, *modelFunctionEntry]{}
	md.Constants = insdict.Dict[ast.ASTHandle, *modelConstantEntry]{}
	m.Budget().Release(md.ownerBytes)
	md.ownerBytes = 0
	md.Closed = true
	return nil
}
