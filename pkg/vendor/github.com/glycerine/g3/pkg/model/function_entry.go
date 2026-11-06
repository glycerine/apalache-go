package model

// Source: src/model/func_interp.cpp:24-68; src/model/func_interp.h:38-64.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"reflect"
)

// FunctionEntry owns its ground argument handles and result. Source flexible
// pointer storage is represented by a charged concrete handle array.
type FunctionEntry struct {
	self                *FunctionEntry
	manager             *ast.ASTManager
	args                []ast.ASTHandle
	result, replacement ast.ASTHandle
	argsAreValues       bool
	ownedArgs, drain    uint32
	resultOwned         bool
	bytes               uint64
	closed              bool
}

func newFunctionEntry(m *ast.ASTManager, args []ast.ASTHandle, result ast.ASTHandle) (*FunctionEntry, error) {
	bytes := uint64(reflect.TypeOf(FunctionEntry{}).Size()) + uint64(len(args))*uint64(reflect.TypeOf(ast.ASTHandle{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	f := &FunctionEntry{manager: m, args: make([]ast.ASTHandle, len(args)), argsAreValues: true, bytes: bytes}
	f.self = f
	if ast.ASTSourceDebug {
		ground, e := m.IsGround(result)
		if e != nil {
			return f, e
		}
		if !ground {
			return f, ast.ErrASTDomain
		}
	}
	if e := m.IncRef(result); e != nil {
		return f, e
	}
	f.result = result
	f.resultOwned = true
	for i, arg := range args {
		if ast.ASTSourceDebug {
			ground, e := m.IsGround(arg)
			if e != nil {
				return f, e
			}
			if !ground {
				return f, ast.ErrASTDomain
			}
		}
		value, e := m.IsValue(arg)
		if e != nil {
			return f, e
		}
		if !value {
			f.argsAreValues = false
		}
		if e = m.IncRef(arg); e != nil {
			return f, e
		}
		f.args[i] = arg
		f.ownedArgs++
	}
	return f, nil
}
func (f *FunctionEntry) open() error {
	if f == nil || f.self != f {
		return ast.ErrASTHandle
	}
	if f.closed {
		return arena.ErrClosed
	}
	if !f.replacement.IsNull() {
		return ast.ErrASTPending
	}
	_, e := f.manager.ProofsEnabled()
	return e
}
func (f *FunctionEntry) Result() (ast.ASTHandle, error) {
	if e := f.open(); e != nil {
		return ast.ASTHandle{}, e
	}
	return f.result, nil
}
func (f *FunctionEntry) Argument(i uint32) (ast.ASTHandle, error) {
	if e := f.open(); e != nil {
		return ast.ASTHandle{}, e
	}
	if uint64(i) >= uint64(len(f.args)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return f.args[i], nil
}
func (f *FunctionEntry) ArgumentsAreValues() bool { return f.argsAreValues }
func (f *FunctionEntry) equalArgs(args []ast.ASTHandle) (bool, error) {
	return f.equalArguments(ast.SliceArguments(args))
}
func (f *FunctionEntry) equalArguments(args ast.AppArguments) (bool, error) {
	if e := f.open(); e != nil {
		return false, e
	}
	if uint64(args.Count()) != uint64(len(f.args)) {
		return false, ast.ErrASTDomain
	}
	for i := uint32(0); i < args.Count(); i++ {
		arg, e := args.At(i)
		if e != nil {
			return false, e
		}
		same, e := f.manager.AreEqual(f.args[i], arg)
		if e != nil || !same {
			return false, e
		}
	}
	return true, nil
}
func (f *FunctionEntry) setResult(value ast.ASTHandle) error {
	if e := f.open(); e != nil {
		return e
	}
	if !f.replacement.IsNull() {
		return arena.ErrClosed
	}
	if e := f.manager.IncRef(value); e != nil {
		return e
	}
	f.replacement = value
	consumed, e := f.manager.DecRefConsumed(f.result)
	if consumed {
		f.resultOwned = false
		f.result = ast.ASTHandle{}
	}
	if e != nil {
		return e
	}
	if !consumed {
		return ast.ErrASTPending
	}
	f.result = value
	f.resultOwned = true
	f.replacement = ast.ASTHandle{}
	return nil
}
func (f *FunctionEntry) close() error {
	if f == nil || f.self != f {
		return ast.ErrASTHandle
	}
	if f.closed {
		return nil
	}
	for f.drain < f.ownedArgs {
		consumed, e := f.manager.DecRefConsumed(f.args[f.drain])
		if consumed {
			f.args[f.drain] = ast.ASTHandle{}
			f.drain++
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	if f.resultOwned {
		consumed, e := f.manager.DecRefConsumed(f.result)
		if consumed {
			f.resultOwned = false
			f.result = ast.ASTHandle{}
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	if !f.replacement.IsNull() {
		consumed, e := f.manager.DecRefConsumed(f.replacement)
		if consumed {
			f.replacement = ast.ASTHandle{}
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	f.manager.Budget().Release(f.bytes)
	f.bytes = 0
	f.args = nil
	f.closed = true
	return nil
}
