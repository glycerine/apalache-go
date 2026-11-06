package api

// Source: generated api_commands.cpp and src/api/api_ast_map.cpp.

import (
	"errors"
	"reflect"
)

type replayASTMap struct {
	object apiObject
	owner  *replayContextOwner
	value  *ASTMap
}

func (m *replayASTMap) apiDestroy() error { return m.value.Close() }
func registerZ3GeneratedMapCommands(r *Z3Replayer) error {
	entries := []struct {
		id   uint32
		name string
	}{
		{457, "Z3_mk_ast_map"}, {458, "Z3_ast_map_inc_ref"}, {459, "Z3_ast_map_dec_ref"}, {460, "Z3_ast_map_contains"},
		{461, "Z3_ast_map_find"}, {462, "Z3_ast_map_insert"}, {463, "Z3_ast_map_erase"}, {464, "Z3_ast_map_reset"},
		{465, "Z3_ast_map_size"}, {466, "Z3_ast_map_keys"}, {467, "Z3_ast_map_to_string"},
	}
	for _, entry := range entries {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error { return replayMapCommand(r, id) }); err != nil {
			return err
		}
	}
	return nil
}
func replayStoreMap(r *Z3Replayer, owner *replayContextOwner, value *ASTMap) error {
	if r.nextObjectID == ^uint64(0) {
		return errors.Join(ErrZ3ReplayState, value.Close())
	}
	n := uint64(reflect.TypeOf(replayASTMap{}).Size())
	if err := owner.results.manager.Budget().Reserve(n); err != nil {
		return errors.Join(err, value.Close())
	}
	m := &replayASTMap{owner: owner, value: value}
	m.object = apiObject{context: owner.results, destructor: m, ownerBytes: n}
	m.object.self = &m.object
	id, err := owner.results.addObject(&m.object)
	if err != nil {
		owner.results.manager.Budget().Release(n)
		return errors.Join(err, value.Close())
	}
	m.object.id = id
	if err = owner.results.saveObject(&m.object); err != nil {
		return err
	}
	if r.maps == nil {
		r.maps = make(map[uint64]*replayASTMap)
	}
	r.nextObjectID++
	replayID := r.nextObjectID
	r.maps[replayID] = m
	owner.mapIDs = append(owner.mapIDs, replayID)
	return r.StoreResult(replayID)
}
func replayMapCommand(r *Z3Replayer, id uint32) (err error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	if id == 457 {
		value, e := owner.context.NewASTMap()
		if e != nil {
			return e
		}
		return replayStoreMap(r, owner, value)
	}
	object, err := r.Object(1)
	if err != nil {
		return err
	}
	if object == 0 && (id == 458 || id == 459) {
		return nil
	}
	m := r.maps[object]
	if m == nil || m.owner != owner || m.object.open() != nil {
		return ErrZ3ReplayReference
	}
	switch id {
	case 458:
		return m.object.incRef()
	case 459:
		return m.object.decRef()
	case 460, 461, 462, 463:
		key, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		switch id {
		case 460:
			_, err = m.value.Contains(key.handle)
		case 461:
			value, e := m.value.Find(key.handle)
			if e != nil {
				return e
			}
			return r.storeBorrowedAST(owner, value)
		case 462:
			value, e := r.astObject(owner, 3)
			if e != nil {
				return e
			}
			err = m.value.Insert(key.handle, value.handle)
		case 463:
			err = m.value.Erase(key.handle)
		}
	case 464:
		err = m.value.Reset()
	case 465:
		_, err = m.value.Size()
	case 466:
		keys, e := m.value.Keys()
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, keys.Close()) }()
		values, e := keys.Values()
		if e != nil {
			return e
		}
		vector, e := owner.context.NewASTVectorValues(values...)
		if e != nil {
			return e
		}
		return replayStoreVector(r, owner, vector)
	case 467:
		_, err = m.value.String()
	}
	return err
}
