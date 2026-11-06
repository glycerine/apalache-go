package arith

// Source: src/util/obj_hashtable.h:58-154; src/util/hashtable.h:190-230,
// 244-255,285-307,327-395. Concrete live obj_map<expr,expr*> specialization
// used by src/tactic/arith/nla2bv_tactic.cpp:126-142.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

type bv2IntPowerEntry struct {
	key, value ast.ASTHandle
	hash       uint32
}

func bv2IntPowerSlot(table []bv2IntPowerEntry, key ast.ASTHandle, hash uint32) int {
	mask := uint32(len(table) - 1)
	for slot := hash & mask; ; slot = (slot + 1) & mask {
		if table[slot].key.IsNull() || table[slot].key == key {
			return int(slot)
		}
	}
}

func (r *BV2IntContext) resizePower2(capacity uint32) error {
	bytes := uint64(capacity) * uint64(reflect.TypeOf(bv2IntPowerEntry{}).Size())
	if err := r.manager.Budget().Reserve(bytes); err != nil {
		return err
	}
	table := make([]bv2IntPowerEntry, capacity)
	// Rehash in the source's old bucket order, including wrapped collisions.
	for _, entry := range r.power2 {
		if !entry.key.IsNull() {
			table[bv2IntPowerSlot(table, entry.key, entry.hash)] = entry
		}
	}
	r.manager.Budget().Release(r.power2Bytes)
	r.power2, r.power2Bytes = table, bytes
	return nil
}

func (r *BV2IntContext) insertPower2(key, value ast.ASTHandle) error {
	// The source checks load before lookup, even for replacement of a key.
	if uint64(r.power2Size)*4 > uint64(len(r.power2))*3 {
		if len(r.power2) >= 1<<31 {
			return arena.ErrBounds
		}
		if err := r.resizePower2(uint32(len(r.power2)) * 2); err != nil {
			return err
		}
	}
	hash, err := r.manager.GetHash(key)
	if err != nil {
		return err
	}
	slot := bv2IntPowerSlot(r.power2, key, hash)
	if r.power2[slot].key.IsNull() {
		r.power2Size++
	}
	r.power2[slot] = bv2IntPowerEntry{key: key, value: value, hash: hash}
	return nil
}

func (r *BV2IntContext) findPower2(key ast.ASTHandle) (ast.ASTHandle, bool, error) {
	hash, err := r.manager.GetHash(key)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	entry := r.power2[bv2IntPowerSlot(r.power2, key, hash)]
	return entry.value, !entry.key.IsNull(), nil
}

func (r *BV2IntContext) power2Entries() ([]bv2IntPowerEntry, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	result := make([]bv2IntPowerEntry, 0, r.power2Size)
	for _, entry := range r.power2 {
		if !entry.key.IsNull() {
			result = append(result, entry)
		}
	}
	return result, nil
}

func (r *BV2IntContext) resetPower2() error {
	if r.power2Size == 0 {
		return nil
	}
	capacity := uint32(len(r.power2))
	if capacity > 16 && uint64(capacity-r.power2Size)*4 > uint64(capacity)*3 {
		// Reserve replacement before releasing storage so failed cleanup can be
		// retried by Reset/Close without losing the owner of the old allocation.
		bytes := uint64(capacity/2) * uint64(reflect.TypeOf(bv2IntPowerEntry{}).Size())
		if err := r.manager.Budget().Reserve(bytes); err != nil {
			return err
		}
		r.manager.Budget().Release(r.power2Bytes)
		r.power2, r.power2Bytes = make([]bv2IntPowerEntry, capacity/2), bytes
	} else {
		clear(r.power2)
	}
	r.power2Size = 0
	return nil
}
