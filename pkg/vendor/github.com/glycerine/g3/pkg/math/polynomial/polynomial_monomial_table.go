package polynomial

// Source: src/math/polynomial/polynomial.cpp; src/math/polynomial/polynomial.h.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/insdict"
)

const (
	monomialTemporaryKey         uint32 = 1
	monomialObjectKey            uint32 = 2
	monomialBorrowedTemporaryKey uint32 = 3
)

type monomialKey struct{ kind, slot uint32 }
type monomialTableEntry struct {
	key  monomialKey
	hash uint32
}

// Entries borrow their handles. Stable entry IDs preserve a temporary's factory
// publication until it is replaced by the completed object. Dictionary order
// makes rename deterministic without reproducing C++ hash bucket geometry.
type monomialTable struct {
	entries insdict.Dict[uint32, monomialTableEntry]
	buckets map[uint32][]uint32
	nextID  uint32
}

func (t *monomialTable) key(i uint32) (monomialKey, error) {
	e, ok := t.entries.Get2(i)
	if !ok {
		return monomialKey{}, arena.ErrBounds
	}
	return e.key, nil
}
func (t *monomialTable) setKey(i uint32, k monomialKey) error {
	e, ok := t.entries.Get2(i)
	if !ok {
		return arena.ErrBounds
	}
	e.key = k
	t.entries.Set(i, e)
	return nil
}
func (t *monomialTable) equal(m *MonomialManager, a, b monomialKey) (bool, error) {
	na, e := m.keyWord(a, 3)
	if e != nil {
		return false, e
	}
	nb, e := m.keyWord(b, 3)
	if e != nil {
		return false, e
	}
	if na != nb {
		return false, nil
	}
	ha, e := m.keyWord(a, 4)
	if e != nil {
		return false, e
	}
	hb, e := m.keyWord(b, 4)
	if e != nil {
		return false, e
	}
	if ha != hb {
		return false, nil
	}
	// Source deliberately excludes total_degree: temporary values do not update it.
	for i := uint32(0); i < na; i++ {
		pa, e := m.keyPower(a, i)
		if e != nil {
			return false, e
		}
		pb, e := m.keyPower(b, i)
		if e != nil {
			return false, e
		}
		if pa != pb {
			return false, nil
		}
	}
	return true, nil
}

func (t *monomialTable) find(m *MonomialManager, k monomialKey) (uint32, bool, error) {
	h, e := m.keyWord(k, 4)
	if e != nil {
		return 0, false, e
	}
	return t.findHashed(m, k, h)
}
func (t *monomialTable) findHashed(m *MonomialManager, k monomialKey, h uint32) (uint32, bool, error) {
	for _, id := range t.buckets[h] {
		entry := t.entries.Get(id)
		same, e := t.equal(m, entry.key, k)
		if e != nil {
			return 0, false, e
		}
		if same {
			return id, true, nil
		}
	}
	return 0, false, nil
}

// Source monomial_manager::rename uses chashtable::insert, which replaces an
// equal key. Factory interning instead keeps the prior canonical object.
func (t *monomialTable) insertRenamed(m *MonomialManager, k monomialKey) error {
	id, inserted, e := t.insert(m, k)
	if e != nil || inserted {
		return e
	}
	return t.setKey(id, k)
}

func (t *monomialTable) insert(m *MonomialManager, k monomialKey) (uint32, bool, error) {
	h, e := m.keyWord(k, 4)
	if e != nil {
		return 0, false, e
	}
	id, found, e := t.findHashed(m, k, h)
	if e != nil || found {
		return id, false, e
	}
	if t.nextID == ^uint32(0) {
		return 0, false, arena.ErrSlotLimit
	}
	id = t.nextID
	t.nextID++
	if t.buckets == nil {
		t.buckets = make(map[uint32][]uint32)
	}
	t.entries.Set(id, monomialTableEntry{k, h})
	t.buckets[h] = append(t.buckets[h], id)
	return id, true, nil
}
func (t *monomialTable) removeID(id uint32) {
	entry, ok := t.entries.Get2(id)
	if !ok {
		return
	}
	bucket := t.buckets[entry.hash]
	for i, existing := range bucket {
		if existing == id {
			copy(bucket[i:], bucket[i+1:])
			bucket = bucket[:len(bucket)-1]
			break
		}
	}
	if len(bucket) == 0 {
		delete(t.buckets, entry.hash)
	} else {
		t.buckets[entry.hash] = bucket
	}
	t.entries.Del(id)
}
func (t *monomialTable) erase(m *MonomialManager, k monomialKey) error {
	id, found, e := t.find(m, k)
	if e != nil {
		return e
	}
	if found {
		t.removeID(id)
	}
	return nil
}

// Owning abort removes the named token using its stored hash even if a partial
// source rename changed the object. It does not repair that source mutation.
func (t *monomialTable) removeToken(k monomialKey) {
	for id, entry := range t.entries.All() {
		if entry.key == k {
			t.removeID(id)
			return
		}
	}
}
func (t *monomialTable) cleanup() { *t = monomialTable{} }
