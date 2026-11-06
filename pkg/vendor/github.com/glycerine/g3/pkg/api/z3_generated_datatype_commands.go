package api

// Source: src/api/api_datatype.cpp and generated api_commands.cpp, datatype
// constructors with multiple AST results.

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func replaySymbolValues(r *Z3Replayer, owner *replayContextOwner, position, n uint32) ([]usymbol.Symbol, error) {
	v, err := r.value(position, replaySymbolArray)
	if err != nil {
		return nil, err
	}
	row := r.symbolArrays[v.u]
	if uint64(n) > uint64(len(row)) {
		return nil, ErrZ3ReplayReference
	}
	result := make([]usymbol.Symbol, n)
	for i := range result {
		result[i], err = replaySymbolValue(owner, row[i])
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
func replayOutputArray(r *Z3Replayer, position, n uint32) ([]uint64, error) {
	values, err := r.ObjectArray(position)
	if err != nil {
		return nil, err
	}
	if uint64(n) > uint64(len(values)) {
		return nil, ErrZ3ReplayReference
	}
	return values[:n], nil
}
func replayMultipleAST(r *Z3Replayer, owner *replayContextOwner, h ast.ASTHandle) (uint64, error) {
	if err := owner.results.saveMultipleASTTrail(h); err != nil {
		return 0, err
	}
	old := r.result
	if err := r.storeBorrowedAST(owner, h); err != nil {
		return 0, err
	}
	id := r.result
	r.result = old
	return id, nil
}
func replayTupleSort(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	name, err := replayTypedSymbol(r, owner, 1)
	if err != nil {
		return err
	}
	n, err := r.Uint(2)
	if err != nil {
		return err
	}
	names, err := replaySymbolValues(r, owner, 3, n)
	if err != nil {
		return err
	}
	sorts, err := replayASTArray(r, owner, 2, 4)
	if err != nil {
		return err
	}
	if _, err = r.Object(5); err != nil {
		return err
	}
	outputs, err := replayOutputArray(r, 6, n)
	if err != nil {
		return err
	}
	if err = owner.results.resetLastResult(); err != nil {
		return err
	}
	tuple, err := owner.context.TupleSortWithSymbol(name, names, sorts)
	if err != nil {
		return err
	}
	id, err := replayMultipleAST(r, owner, tuple.Sort)
	if err != nil {
		return err
	}
	constructor, err := replayMultipleAST(r, owner, tuple.Constructor)
	if err != nil {
		return err
	}
	if err = r.SetObject(5, constructor); err != nil {
		return err
	}
	for i, h := range tuple.Fields {
		outputs[i], err = replayMultipleAST(r, owner, h)
		if err != nil {
			return err
		}
	}
	return r.StoreResult(id)
}
func replayEnumerationSort(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	name, err := replayTypedSymbol(r, owner, 1)
	if err != nil {
		return err
	}
	n, err := r.Uint(2)
	if err != nil {
		return err
	}
	names, err := replaySymbolValues(r, owner, 3, n)
	if err != nil {
		return err
	}
	constants, err := replayOutputArray(r, 4, n)
	if err != nil {
		return err
	}
	testers, err := replayOutputArray(r, 5, n)
	if err != nil {
		return err
	}
	if err = owner.results.resetLastResult(); err != nil {
		return err
	}
	enumeration, err := owner.context.EnumerationSortWithSymbol(name, names)
	if err != nil {
		return err
	}
	id, err := replayMultipleAST(r, owner, enumeration.Sort)
	if err != nil {
		return err
	}
	for i := range constants {
		constants[i], err = replayMultipleAST(r, owner, enumeration.Constants[i])
		if err != nil {
			return err
		}
		testers[i], err = replayMultipleAST(r, owner, enumeration.Recognizers[i])
		if err != nil {
			return err
		}
	}
	return r.StoreResult(id)
}
func replayListSort(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	name, err := replayTypedSymbol(r, owner, 1)
	if err != nil {
		return err
	}
	element, err := r.astObject(owner, 2)
	if err != nil {
		return err
	}
	for i := uint32(3); i <= 8; i++ {
		if _, err = r.Object(i); err != nil {
			return err
		}
	}
	if err = owner.results.resetLastResult(); err != nil {
		return err
	}
	list, err := owner.context.ListSortWithSymbol(name, element.handle)
	if err != nil {
		return err
	}
	id, err := replayMultipleAST(r, owner, list.Sort)
	if err != nil {
		return err
	}
	for i, h := range []ast.ASTHandle{list.Nil, list.IsNil, list.Cons, list.IsCons, list.Head, list.Tail} {
		value, e := replayMultipleAST(r, owner, h)
		if e != nil {
			return e
		}
		if e = r.SetObject(uint32(i+3), value); e != nil {
			return e
		}
	}
	return r.StoreResult(id)
}
