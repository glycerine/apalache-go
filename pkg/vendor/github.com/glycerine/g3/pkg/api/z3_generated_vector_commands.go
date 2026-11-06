package api

// Source: generated api_commands.cpp and src/api/api_ast_vector.cpp.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

type replayASTVector struct {
	object apiObject
	owner  *replayContextOwner
	value  *ASTVector
}

func (v *replayASTVector) apiDestroy() error { return v.value.Close() }
func registerZ3GeneratedVectorCommands(r *Z3Replayer) error {
	entries := []struct {
		id   uint32
		name string
	}{
		{447, "Z3_mk_ast_vector"}, {448, "Z3_ast_vector_inc_ref"}, {449, "Z3_ast_vector_dec_ref"},
		{450, "Z3_ast_vector_size"}, {451, "Z3_ast_vector_get"}, {452, "Z3_ast_vector_set"},
		{453, "Z3_ast_vector_resize"}, {454, "Z3_ast_vector_push"}, {455, "Z3_ast_vector_translate"}, {456, "Z3_ast_vector_to_string"},
	}
	for _, entry := range entries {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error { return replayVectorCommand(r, id) }); err != nil {
			return err
		}
	}
	return nil
}
func replayStoreVector(r *Z3Replayer, owner *replayContextOwner, value *ASTVector) error {
	if r.nextObjectID == ^uint64(0) {
		return errors.Join(ErrZ3ReplayState, value.Close())
	}
	n := uint64(reflect.TypeOf(replayASTVector{}).Size())
	if err := owner.results.manager.Budget().Reserve(n); err != nil {
		return errors.Join(err, value.Close())
	}
	v := &replayASTVector{owner: owner, value: value}
	v.object = apiObject{context: owner.results, destructor: v, ownerBytes: n}
	v.object.self = &v.object
	id, err := owner.results.addObject(&v.object)
	if err != nil {
		owner.results.manager.Budget().Release(n)
		return errors.Join(err, value.Close())
	}
	v.object.id = id
	if err = owner.results.saveObject(&v.object); err != nil {
		return err
	}
	if r.vectors == nil {
		r.vectors = make(map[uint64]*replayASTVector)
	}
	r.nextObjectID++
	replayID := r.nextObjectID
	r.vectors[replayID] = v
	owner.vectorIDs = append(owner.vectorIDs, replayID)
	return r.StoreResult(replayID)
}
func replayGetVector(r *Z3Replayer) (*replayASTVector, error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return nil, err
	}
	return replayVectorAt(r, owner, 1)
}
func replayVectorAt(r *Z3Replayer, owner *replayContextOwner, position uint32) (*replayASTVector, error) {
	id, err := r.Object(position)
	if err != nil {
		return nil, err
	}
	v := r.vectors[id]
	if v == nil || v.owner != owner || v.object.open() != nil {
		return nil, ErrZ3ReplayReference
	}
	return v, nil
}
func replayVectorCommand(r *Z3Replayer, id uint32) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	if id == 447 {
		value, e := owner.context.NewASTVector()
		if e != nil {
			return e
		}
		return replayStoreVector(r, owner, value)
	}
	if id == 448 || id == 449 {
		object, e := r.Object(1)
		if e != nil || object == 0 {
			return e
		}
	}
	v, err := replayGetVector(r)
	if err != nil {
		return err
	}
	switch id {
	case 448:
		return v.object.incRef()
	case 449:
		return v.object.decRef()
	case 450:
		_, err = v.value.Len()
	case 451:
		var index uint32
		index, err = r.Uint(2)
		if err != nil {
			return err
		}
		var result ast.ASTHandle
		result, err = v.value.At(index)
		if err != nil {
			return err
		}
		if result.IsNull() {
			return r.StoreResult(0)
		}
		return r.storeBorrowedAST(owner, result)
	case 452, 454:
		position := uint32(2)
		var index uint32
		if id == 452 {
			index, err = r.Uint(2)
			if err != nil {
				return err
			}
			position = 3
		}
		var object uint64
		object, err = r.Object(position)
		if err != nil {
			return err
		}
		var value ast.ASTHandle
		if object != 0 {
			var entry *replayASTOwner
			entry, err = r.astByID(owner, object)
			if err != nil {
				return err
			}
			value = entry.handle
		}
		if id == 452 {
			err = v.value.Set(index, value)
		} else {
			err = v.value.Push(value)
		}
	case 453:
		var size uint32
		size, err = r.Uint(2)
		if err == nil {
			err = v.value.Resize(size)
		}
	case 455:
		var target *replayContextOwner
		target, err = replayContext(r, 2)
		if err != nil {
			return err
		}
		value, e := v.value.Translate(target.context)
		if e != nil {
			return e
		}
		return replayStoreVector(r, target, value)
	case 456:
		_, err = v.value.String()
	}
	return err
}
