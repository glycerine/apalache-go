package ast

// Source: src/ast/ast.h:920-924; src/ast/ast.cpp:1714-1780.

import "github.com/glycerine/insdict"

// Canonical entries borrow exact node tokens. Native hash buckets accelerate
// structural comparison; dictionary order makes teardown deterministic.
type astCanonicalTable struct {
	entries insdict.Dict[uint64, astNodeRef]
	buckets map[uint32][]astNodeRef
	size    uint32
}

func astTableKey(r astNodeRef) uint64 {
	return uint64(r.slot)<<32 | uint64(r.generation)
}

func (m *ASTManager) initTable() {
	m.table = astCanonicalTable{buckets: make(map[uint32][]astNodeRef)}
}

func (m *ASTManager) tableFind(key astNodeRef) (astNodeRef, bool, error) {
	hash := m.nodeAt(key.slot).hash
	for _, other := range m.table.buckets[hash] {
		if e := m.validNode(other); e != nil {
			return astNodeRef{}, false, e
		}
		equal, e := m.compareNodes(other, key)
		if e != nil {
			return astNodeRef{}, false, e
		}
		if equal {
			return other, true, nil
		}
	}
	return astNodeRef{}, false, nil
}

func (m *ASTManager) tableInsert(key astNodeRef) (astNodeRef, bool, error) {
	if old, found, e := m.tableFind(key); e != nil || found {
		return old, false, e
	}
	hash := m.nodeAt(key.slot).hash
	m.table.buckets[hash] = append(m.table.buckets[hash], key)
	m.table.entries.Put(astTableKey(key), key)
	m.table.size++
	return key, true, nil
}

func (m *ASTManager) tableEraseIdentity(key astNodeRef) error {
	return m.tableEraseCaptured(key, m.nodeAt(key.slot).hash)
}

// Matching captured erasure never dereferences a source-freed placement or
// a replacement generation. Only the original exact token is consumed.
func (m *ASTManager) tableEraseCaptured(key astNodeRef, hash uint32) error {
	bucket := m.table.buckets[hash]
	for i, other := range bucket {
		if other == key {
			copy(bucket[i:], bucket[i+1:])
			bucket = bucket[:len(bucket)-1]
			if len(bucket) == 0 {
				delete(m.table.buckets, hash)
			} else {
				m.table.buckets[hash] = bucket
			}
			m.table.entries.DelPackMaybe(astTableKey(key))
			m.table.size--
			return nil
		}
	}
	return ErrASTHandle
}
