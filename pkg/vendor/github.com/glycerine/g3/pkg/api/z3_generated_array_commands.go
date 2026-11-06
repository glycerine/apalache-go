package api

// Source: generated api_commands.cpp, array and finite-domain construction.

import "github.com/glycerine/g3/pkg/ast"

func replayFiniteDomainSort(r *Z3Replayer) error {
	owner, e := replayContext(r, 0)
	if e != nil {
		return e
	}
	name, e := replayTypedSymbol(r, owner, 1)
	if e != nil {
		return e
	}
	size, e := r.Uint64(2)
	if e != nil {
		return e
	}
	h, e := owner.results.manager.MkDatalogFiniteSort(name, size)
	if e != nil {
		return e
	}
	return r.storeAST(owner, h)
}
func replayArraySort(r *Z3Replayer) error {
	owner, e := replayContext(r, 0)
	if e != nil {
		return e
	}
	domain, e := r.astObject(owner, 1)
	if e != nil {
		return e
	}
	target, e := r.astObject(owner, 2)
	if e != nil {
		return e
	}
	h, e := owner.context.ArraySort([]ast.ASTHandle{domain.handle}, target.handle)
	if e != nil {
		return e
	}
	return r.storeAST(owner, h)
}
func replayArraySortN(r *Z3Replayer) error {
	owner, e := replayContext(r, 0)
	if e != nil {
		return e
	}
	domain, e := replayASTArray(r, owner, 1, 2)
	if e != nil {
		return e
	}
	target, e := r.astObject(owner, 3)
	if e != nil {
		return e
	}
	h, e := owner.context.ArraySort(domain, target.handle)
	if e != nil {
		return e
	}
	return r.storeAST(owner, h)
}
func replayArrayOp(r *Z3Replayer, many, store bool) error {
	owner, e := replayContext(r, 0)
	if e != nil {
		return e
	}
	array, e := r.astObject(owner, 1)
	if e != nil {
		return e
	}
	var indices []ast.ASTHandle
	valuePosition := uint32(3)
	if many {
		indices, e = replayASTArray(r, owner, 2, 3)
		valuePosition = 4
	} else {
		var index *replayASTOwner
		index, e = r.astObject(owner, 2)
		if e == nil {
			indices = []ast.ASTHandle{index.handle}
		}
	}
	if e != nil {
		return e
	}
	var h ast.ASTHandle
	if store {
		value, err := r.astObject(owner, valuePosition)
		if err != nil {
			return err
		}
		h, e = owner.context.ArrayStore(array.handle, indices, value.handle)
	} else {
		h, e = owner.context.ArraySelect(array.handle, indices...)
	}
	if e != nil {
		return e
	}
	return r.storeAST(owner, h)
}
func replaySelect(r *Z3Replayer) error  { return replayArrayOp(r, false, false) }
func replaySelectN(r *Z3Replayer) error { return replayArrayOp(r, true, false) }
func replayStore(r *Z3Replayer) error   { return replayArrayOp(r, false, true) }
func replayStoreN(r *Z3Replayer) error  { return replayArrayOp(r, true, true) }
func replayConstArray(r *Z3Replayer) error {
	owner, e := replayContext(r, 0)
	if e != nil {
		return e
	}
	domain, e := r.astObject(owner, 1)
	if e != nil {
		return e
	}
	value, e := r.astObject(owner, 2)
	if e != nil {
		return e
	}
	rangeSort, e := owner.context.SortOf(value.handle)
	if e != nil {
		return e
	}
	arraySort, e := owner.context.ArraySort([]ast.ASTHandle{domain.handle}, rangeSort)
	if e != nil {
		return e
	}
	h, e := owner.context.ConstantArray(arraySort, value.handle)
	if e != nil {
		return e
	}
	return r.storeAST(owner, h)
}
func replayArrayMap(r *Z3Replayer) error {
	owner, e := replayContext(r, 0)
	if e != nil {
		return e
	}
	decl, e := r.astObject(owner, 1)
	if e != nil {
		return e
	}
	args, e := replayASTArray(r, owner, 2, 3)
	if e != nil {
		return e
	}
	h, e := owner.context.ArrayMap(decl.handle, args...)
	if e != nil {
		return e
	}
	return r.storeAST(owner, h)
}
func replayArrayDefault(r *Z3Replayer) error {
	owner, e := replayContext(r, 0)
	if e != nil {
		return e
	}
	array, e := r.astObject(owner, 1)
	if e != nil {
		return e
	}
	h, e := owner.context.ArrayDefault(array.handle)
	if e != nil {
		return e
	}
	return r.storeAST(owner, h)
}

func replayAsArray(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	declaration, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	value, err := owner.context.AsArray(declaration.handle)
	if err != nil {
		return err
	}
	return r.storeAST(owner, value)
}
