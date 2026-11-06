package api

// Source: generated api_commands.cpp, uninterpreted sort and declaration commands.

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func replayASTArray(r *Z3Replayer, owner *replayContextOwner, countPosition, arrayPosition uint32) ([]ast.ASTHandle, error) {
	n, err := r.Uint(countPosition)
	if err != nil {
		return nil, err
	}
	ids, err := r.ObjectArray(arrayPosition)
	if err != nil {
		return nil, err
	}
	if uint64(n) > uint64(len(ids)) {
		return nil, ErrZ3ReplayReference
	}
	args := make([]ast.ASTHandle, n)
	for i := range args {
		entry, err := r.astByID(owner, ids[i])
		if err != nil {
			return nil, err
		}
		args[i] = entry.handle
	}
	return args, nil
}
func replayTypedSymbol(r *Z3Replayer, owner *replayContextOwner, position uint32) (usymbol.Symbol, error) {
	v, err := r.value(position, replaySymbol)
	if err != nil {
		return usymbol.Symbol{}, err
	}
	return replaySymbolValue(owner, v)
}
func replaySymbolValue(owner *replayContextOwner, v z3ReplayValue) (usymbol.Symbol, error) {
	if v.nullSymbol {
		return usymbol.Symbol{}, nil
	}
	if v.numericSymbol {
		return usymbol.Numerical(uint32(v.u)), nil
	}
	return owner.context.StringSymbol(v.s)
}
func replayUninterpretedSort(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	name, err := replayTypedSymbol(r, owner, 1)
	if err != nil {
		return err
	}
	h, err := owner.results.manager.MkUninterpretedSort(name, nil)
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayFunction(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	name, err := replayTypedSymbol(r, owner, 1)
	if err != nil {
		return err
	}
	domain, err := replayASTArray(r, owner, 2, 3)
	if err != nil {
		return err
	}
	target, err := r.astObject(owner, 4)
	if err != nil {
		return err
	}
	h, err := owner.results.manager.MkFuncDecl(name, domain, target.handle, ast.ASTFuncDeclInfo{})
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayApplication(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	decl, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	args, err := replayASTArray(r, owner, 2, 3)
	if err != nil {
		return err
	}
	h, err := owner.context.Apply(decl.handle, args...)
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayConstant(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	name, err := replayTypedSymbol(r, owner, 1)
	if err != nil {
		return err
	}
	sort, err := r.astObject(owner, 2)
	if err != nil {
		return err
	}
	h, err := owner.results.manager.MkConst(name, sort.handle)
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayFreshFunction(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	prefix, err := r.String(1)
	if err != nil {
		return err
	}
	domain, err := replayASTArray(r, owner, 2, 3)
	if err != nil {
		return err
	}
	target, err := r.astObject(owner, 4)
	if err != nil {
		return err
	}
	h, err := owner.context.FreshFunction(prefix, domain, target.handle)
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayFreshConstant(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	prefix, err := r.String(1)
	if err != nil {
		return err
	}
	sort, err := r.astObject(owner, 2)
	if err != nil {
		return err
	}
	h, err := owner.context.FreshConstant(prefix, sort.handle)
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
