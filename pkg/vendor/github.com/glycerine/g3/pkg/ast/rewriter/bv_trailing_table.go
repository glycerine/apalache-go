package rewriter

// Source: src/ast/rewriter/bv_trailing.cpp:338-379.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/insdict"
)

type bvTrailingEntry struct {
	key      ast.ASTHandle
	min, max uint32
	owned    bool
}

// Each entry owns one key reference. Dictionary order makes stopped cleanup
// deterministic; native storage growth has no numerical/object arena charge.
type bvTrailingTable struct {
	entries insdict.Dict[ast.ASTHandle, bvTrailingEntry]
	header  uint64
}

func bvTrailingHash(key ast.ASTHandle) uint64 {
	return key.Origin() ^ uint64(key.Slot())*0x9e3779b97f4a7c15 ^ uint64(key.Generation())*0xbf58476d1ce4e5b9
}

func newBVTrailingTable(m *ast.ASTManager) (*bvTrailingTable, error) {
	header := uint64(reflect.TypeOf(bvTrailingTable{}).Size())
	if e := m.Budget().Reserve(header); e != nil {
		return nil, e
	}
	return &bvTrailingTable{entries: *insdict.NewDictFunc[ast.ASTHandle, bvTrailingEntry](bvTrailingHash), header: header}, nil
}

func (t *bvTrailingTable) find(m *ast.ASTManager, key ast.ASTHandle) (bvTrailingEntry, bool, error) {
	if _, e := m.GetHash(key); e != nil {
		return bvTrailingEntry{}, false, e
	}
	entry, found := t.entries.Get2(key)
	return entry, found, nil
}

func (t *bvTrailingTable) insert(key ast.ASTHandle, min, max uint32) {
	entry, found := t.entries.Get2(key)
	if !found {
		entry = bvTrailingEntry{key: key, owned: true}
	}
	entry.min, entry.max = min, max
	t.entries.Put(key, entry)
}

func (t *bvTrailingTable) close(m *ast.ASTManager) error {
	for key, entry := range t.entries.All() {
		if !entry.owned {
			continue
		}
		consumed, e := m.DecRefConsumed(entry.key)
		if consumed {
			entry.owned = false
			t.entries.Set(key, entry)
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	m.Budget().Release(t.header)
	t.entries.Clear()
	t.header = 0
	return nil
}
