package model

// Native tuple lookup for src/model/model_evaluator.cpp:337-416.

import (
	"encoding/binary"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/insdict"
)

// Rows borrow the owning expansion slices. Keys copy only index identities;
// replacing an equal tuple changes its result without changing insertion order.
type arrayTupleTable struct {
	arity int
	rows  insdict.Dict[string, []ast.ASTHandle]
}

func (t *arrayTupleTable) key(row []ast.ASTHandle) (string, error) {
	if t.arity <= 0 || len(row) <= t.arity {
		return "", ast.ErrASTDomain
	}
	// Include complete handle identity, not the AST hash (which can collide).
	key := make([]byte, 0, t.arity*16)
	for _, h := range row[:t.arity] {
		key = binary.LittleEndian.AppendUint64(key, h.Origin())
		key = binary.LittleEndian.AppendUint32(key, h.Slot())
		key = binary.LittleEndian.AppendUint32(key, h.Generation())
	}
	return string(key), nil
}
func (t *arrayTupleTable) find(row []ast.ASTHandle) ([]ast.ASTHandle, bool, error) {
	key, e := t.key(row)
	if e != nil {
		return nil, false, e
	}
	value, found := t.rows.Get2(key)
	return value, found, nil
}
func (t *arrayTupleTable) insert(row []ast.ASTHandle) error {
	key, e := t.key(row)
	if e != nil {
		return e
	}
	t.rows.Set(key, row)
	return nil
}
func (t *arrayTupleTable) remove(row []ast.ASTHandle) error {
	key, e := t.key(row)
	if e != nil {
		return e
	}
	t.rows.Del(key)
	return nil
}
func (t *arrayTupleTable) close() { *t = arrayTupleTable{} }
