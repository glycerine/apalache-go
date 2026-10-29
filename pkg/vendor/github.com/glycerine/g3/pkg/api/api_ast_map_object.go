package api

// Source: src/api/api_ast_map.cpp:27-137; src/api/api_ast_map.h:25-31;
// src/util/dec_ref_util.h:24-36.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

var errAPIASTMapMissing = errors.New("api: AST map key is absent")

type apiASTMapEntry struct {
	value                ast.ASTHandle
	keyOwned, valueOwned bool
}

// apiASTMapObject is the concrete source Z3_ast_map_ref wrapper. It borrows
// the context's AST manager and owns one AST reference for each key and value.
type apiASTMapObject struct {
	object  apiObject
	entries map[ast.ASTHandle]*apiASTMapEntry
	pending []ast.ASTHandle
}

func newAPIASTMapObject(c *apiContextResults) (*apiASTMapObject, error) {
	if e := c.open(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(apiASTMapObject{}).Size())
	if e := c.manager.Budget().Reserve(n); e != nil {
		return nil, e
	}
	a := &apiASTMapObject{}
	c.traceASTMapAllocated(a, n)
	a.entries = make(map[ast.ASTHandle]*apiASTMapEntry)
	a.object = apiObject{context: c, destructor: a, ownerBytes: n}
	a.object.self = &a.object
	id, e := c.addObject(&a.object)
	if e != nil {
		c.manager.Budget().Release(n)
		return nil, e
	}
	a.object.id = id
	return a, nil
}

func (a *apiASTMapObject) check() error {
	if a == nil {
		return errAPIObjectOwner
	}
	return a.object.open()
}

func (a *apiASTMapObject) dec(h ast.ASTHandle) error {
	consumed, e := a.object.context.manager.DecRefConsumed(h)
	if !consumed {
		a.pending = append(a.pending, h)
	}
	return e
}

func (a *apiASTMapObject) insert(key, value ast.ASTHandle) error {
	if e := a.check(); e != nil {
		return e
	}
	m := a.object.context.manager
	if _, e := m.GetKind(key); e != nil {
		return e
	}
	if _, e := m.GetKind(value); e != nil {
		return e
	}
	entry, found := a.entries[key]
	if !found {
		// Source insert_if_not_there2 publishes a null value before INC.
		entry = &apiASTMapEntry{}
		a.entries[key] = entry
	}
	if !entry.valueOwned {
		if !entry.keyOwned {
			if e := m.IncRef(key); e != nil {
				return e
			}
			entry.keyOwned = true
		}
		if e := m.IncRef(value); e != nil {
			return e
		}
		entry.value = value
		entry.valueOwned = true
		return nil
	}
	// Source increments the incoming value before decrementing the old one.
	if e := m.IncRef(value); e != nil {
		return e
	}
	old := entry.value
	if e := a.dec(old); e != nil {
		// The source entry still contains old after a stopped DEC. Keep the
		// acquired incoming reference for explicit Go owner cleanup.
		entry.valueOwned = false
		a.pending = append(a.pending, value)
		return e
	}
	entry.value = value
	entry.valueOwned = true
	return nil
}

func (a *apiASTMapObject) contains(key ast.ASTHandle) (bool, error) {
	if e := a.check(); e != nil {
		return false, e
	}
	_, found := a.entries[key]
	return found, nil
}

func (a *apiASTMapObject) find(key ast.ASTHandle) (ast.ASTHandle, error) {
	if e := a.check(); e != nil {
		return ast.ASTHandle{}, e
	}
	entry, found := a.entries[key]
	if !found {
		return ast.ASTHandle{}, errAPIASTMapMissing
	}
	return entry.value, nil
}

func (a *apiASTMapObject) size() (uint32, error) {
	if e := a.check(); e != nil {
		return 0, e
	}
	return uint32(len(a.entries)), nil
}

// keys borrows the map's AST handles. The future API AST-vector wrapper must
// retain each key before it publishes the vector beyond this map's lifetime.
func (a *apiASTMapObject) keys() ([]ast.ASTHandle, error) {
	if e := a.check(); e != nil {
		return nil, e
	}
	keys := make([]ast.ASTHandle, 0, len(a.entries))
	for key := range a.entries {
		keys = append(keys, key)
	}
	return keys, nil
}

func (a *apiASTMapObject) queueOwned(key ast.ASTHandle, entry *apiASTMapEntry) {
	if entry.keyOwned {
		a.pending = append(a.pending, key)
		entry.keyOwned = false
	}
	if entry.valueOwned {
		a.pending = append(a.pending, entry.value)
		entry.valueOwned = false
	}
}

func (a *apiASTMapObject) erase(key ast.ASTHandle) error {
	if e := a.check(); e != nil {
		return e
	}
	entry, found := a.entries[key]
	if !found {
		return nil
	}
	// Source erases the table entry before decrementing key then value.
	delete(a.entries, key)
	if entry.keyOwned {
		entry.keyOwned = false
		if e := a.dec(key); e != nil {
			a.queueOwned(key, entry)
			return e
		}
	}
	if entry.valueOwned {
		entry.valueOwned = false
		return a.dec(entry.value)
	}
	return nil
}

func (a *apiASTMapObject) reset() error {
	if e := a.check(); e != nil {
		return e
	}
	return a.resetCore()
}

func (a *apiASTMapObject) resetCore() error {
	for key, entry := range a.entries {
		if entry.keyOwned {
			entry.keyOwned = false
			if e := a.dec(key); e != nil {
				return e
			}
		}
		if entry.valueOwned {
			entry.valueOwned = false
			if e := a.dec(entry.value); e != nil {
				return e
			}
		}
		delete(a.entries, key)
	}
	for len(a.pending) != 0 {
		last := len(a.pending) - 1
		h := a.pending[last]
		consumed, e := a.object.context.manager.DecRefConsumed(h)
		if consumed {
			a.pending = a.pending[:last]
		}
		if e != nil {
			return e
		}
		if !consumed {
			return errAPIObjectOwner
		}
	}
	return nil
}

// copyExprPairs is the concrete transfer loop in api_qe.cpp:99-104. The
// source expression map remains the owner until this map increments each pair.
func (a *apiASTMapObject) copyExprPairs(source *ast.ASTExprMap) error {
	if e := a.check(); e != nil {
		return e
	}
	pairs, e := source.Pairs()
	if e != nil {
		return e
	}
	for _, pair := range pairs {
		if e = a.insert(pair.Key, pair.Value); e != nil {
			return e
		}
	}
	return nil
}

func (a *apiASTMapObject) apiDestroy() error {
	if e := a.resetCore(); e != nil {
		return e
	}
	a.entries = nil
	a.pending = nil
	return nil
}
