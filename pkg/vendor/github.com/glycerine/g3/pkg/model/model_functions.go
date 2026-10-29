package model

// Source: src/model/model_core.cpp:21-42,68-109;
// src/model/model_core.h:45-64.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// RegisterFunction transfers graph ownership on success. Replacement destroys
// the old graph before publication; registering the identical pointer is a no-op.
func (md *ModelCore) RegisterFunction(d ast.ASTHandle, fi *FunctionInterpretation) error {
	e := md.checkDeclaration(d)
	if e != nil {
		return e
	}
	if md.removedFunction != nil {
		return arena.ErrClosed
	}
	arity, e := md.manager.GetArity(d)
	if e != nil {
		return e
	}
	if arity == 0 || fi == nil {
		return ast.ErrASTDomain
	}
	if e = fi.open(); e != nil {
		return e
	}
	if fi.manager != md.manager {
		return ast.ErrASTHandle
	}
	if fi.arity != arity {
		return ast.ErrASTDomain
	}
	entry, found := md.Functions.Get2(d)
	if !found {
		entry = &modelFunctionEntry{declaration: d}
		md.Functions.Set(d, entry)
	}
	if entry.function == nil {
		md.declarations = append(md.declarations, d)
		md.functionDeclarations = append(md.functionDeclarations, d)
		if e = md.manager.IncRef(d); e != nil {
			return e
		}
		entry.declarationOwned = true
		entry.function = fi
	} else {
		old := entry.function
		if old != fi {
			if e = old.Close(); e != nil {
				return e
			}
			old.traceEvaluatorFunctionDeallocated(81)
		}
		entry.function = fi
	}
	return nil
}

// FunctionInterpretation returns a borrowed graph owned by the model.
func (md *ModelCore) FunctionInterpretation(d ast.ASTHandle) (*FunctionInterpretation, error) {
	e := md.checkDeclaration(d)
	if e != nil {
		return nil, e
	}
	entry, found := md.Functions.Get2(d)
	if !found {
		return nil, nil
	}
	return entry.function, nil
}
func (md *ModelCore) HasInterpretation(d ast.ASTHandle) (bool, error) {
	e := md.checkDeclaration(d)
	if e != nil {
		return false, e
	}
	_, constant := md.Constants.Get2(d)
	_, function := md.Functions.Get2(d)
	return constant || function, nil
}
func (md *ModelCore) Function(i uint32) (ast.ASTHandle, error) {
	if e := md.open(); e != nil {
		return ast.ASTHandle{}, e
	}
	if uint64(i) >= uint64(len(md.functionDeclarations)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return md.functionDeclarations[i], nil
}

// EvalDeclaration preserves the distinct source misses: absent functions leave
// the output unchanged, while present partial graphs assign their null result.
func (md *ModelCore) EvalDeclaration(d ast.ASTHandle, out ast.ASTRef) (bool, error) {
	if e := md.open(); e != nil {
		return false, e
	}
	arity, e := md.manager.GetArity(d)
	if e != nil {
		return false, e
	}
	if arity == 0 {
		return md.EvalConstant(d, out)
	}
	fi, e := md.FunctionInterpretation(d)
	if e != nil || fi == nil {
		return false, e
	}
	value, e := fi.Interpretation()
	if e != nil {
		return false, e
	}
	if e = md.manager.SetRef(out, value); e != nil {
		return false, e
	}
	return !value.IsNull(), nil
}
func (md *ModelCore) UnregisterDeclaration(d ast.ASTHandle) error {
	e := md.checkDeclaration(d)
	if e != nil {
		return e
	}
	if entry, found := md.Constants.Get2(d); found && !entry.value.IsNull() {
		return md.UnregisterConstant(d)
	}
	if md.removedFunction != nil {
		return arena.ErrClosed
	}
	entry, found := md.Functions.Get2(d)
	if !found || entry.function == nil {
		return nil
	}
	md.removedFunction = entry
	md.Functions.Del(d)
	for i, h := range md.functionDeclarations {
		if h == d {
			copy(md.functionDeclarations[i:], md.functionDeclarations[i+1:])
			md.functionDeclarations[len(md.functionDeclarations)-1] = ast.ASTHandle{}
			md.functionDeclarations = md.functionDeclarations[:len(md.functionDeclarations)-1]
			break
		}
	}
	return md.finishFunctionRemoval()
}
func (md *ModelCore) finishFunctionRemoval() error {
	if md.removedFunction != nil {
		if e := md.retireFunction(md.removedFunction, 105); e != nil {
			return e
		}
		md.removedFunction = nil
	}
	return nil
}
