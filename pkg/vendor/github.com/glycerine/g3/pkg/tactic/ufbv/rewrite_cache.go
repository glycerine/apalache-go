package ufbv

// Source: src/util/array_map.h:29-91, and the concrete specialization in
// src/tactic/ufbv/ufbv_rewriter.h:98-111.

import (
	"errors"
	"math"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

type rewriteCacheValue struct {
	expr ast.ASTHandle
	done bool
}

type rewriteCacheEntry struct {
	key       ast.ASTHandle
	value     rewriteCacheValue
	timestamp uint32
}

// rewriteCache is array_map<expr*, pair<expr*, bool>, plugin>. Its ID-indexed
// entries retain keys and result expressions even after Reset makes them
// invisible; a later replacement or Close releases those references.
type rewriteCache struct {
	manager   *ast.ASTManager
	entries   map[uint32]rewriteCacheEntry
	pending   []ast.ASTHandle
	timestamp uint32
	closed    bool
}

func newRewriteCache(m *ast.ASTManager) (*rewriteCache, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	return &rewriteCache{manager: m, entries: make(map[uint32]rewriteCacheEntry)}, nil
}

func (c *rewriteCache) check() error {
	if c == nil || c.manager == nil || c.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (c *rewriteCache) id(h ast.ASTHandle) (uint32, error) {
	kind, e := c.manager.GetKind(h)
	if e != nil {
		return 0, e
	}
	if kind != ast.ASTApp && kind != ast.ASTVar && kind != ast.ASTQuantifier {
		return 0, ast.ErrASTDomain
	}
	return c.manager.GetASTID(h)
}

func (c *rewriteCache) release(h ast.ASTHandle) error {
	consumed, e := c.manager.DecRefConsumed(h)
	if !consumed {
		c.pending = append(c.pending, h)
	}
	return e
}

func (c *rewriteCache) contains(key ast.ASTHandle) (bool, error) {
	if e := c.check(); e != nil {
		return false, e
	}
	id, e := c.id(key)
	if e != nil {
		return false, e
	}
	entry, found := c.entries[id]
	return found && entry.timestamp == c.timestamp, nil
}

func (c *rewriteCache) get(key ast.ASTHandle) (rewriteCacheValue, bool, error) {
	if e := c.check(); e != nil {
		return rewriteCacheValue{}, false, e
	}
	id, e := c.id(key)
	if e != nil {
		return rewriteCacheValue{}, false, e
	}
	entry, found := c.entries[id]
	if !found || entry.timestamp != c.timestamp {
		return rewriteCacheValue{}, false, nil
	}
	return entry.value, true, nil
}

func (c *rewriteCache) insert(key, value ast.ASTHandle, done bool) error {
	if e := c.check(); e != nil {
		return e
	}
	id, e := c.id(key)
	if e != nil {
		return e
	}
	if _, e = c.id(value); e != nil {
		return e
	}
	// plugin::ins_eh runs before plugin::del_eh, including self replacement.
	if e = c.manager.IncRef(key); e != nil {
		return e
	}
	if e = c.manager.IncRef(value); e != nil {
		return errors.Join(e, c.release(key))
	}
	if old, found := c.entries[id]; found {
		// Publish the replacement before releasing the old entry so cleanup
		// can resume after a manager failure.
		c.entries[id] = rewriteCacheEntry{key: key, value: rewriteCacheValue{value, done}, timestamp: c.timestamp}
		if e = c.release(old.key); e != nil {
			c.pending = append(c.pending, old.value.expr)
			return e
		}
		return c.release(old.value.expr)
	}
	c.entries[id] = rewriteCacheEntry{key: key, value: rewriteCacheValue{value, done}, timestamp: c.timestamp}
	return nil
}

// reset is lazy: source reset does not release retained keys or values.
func (c *rewriteCache) reset() error {
	if e := c.check(); e != nil {
		return e
	}
	if c.timestamp == math.MaxUint32 {
		if e := c.reallyFlush(); e != nil {
			return e
		}
		c.timestamp = 0
	} else {
		c.timestamp++
	}
	return nil
}

func (c *rewriteCache) reallyFlush() error {
	ids := make([]uint32, 0, len(c.entries))
	for id := range c.entries {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] }) // source optional-entry vector is visited by ascending ID
	for _, id := range ids {
		entry := c.entries[id]
		delete(c.entries, id)
		if e := c.release(entry.key); e != nil {
			c.pending = append(c.pending, entry.value.expr)
			return e
		}
		if e := c.release(entry.value.expr); e != nil {
			return e
		}
	}
	for len(c.pending) > 0 {
		last := len(c.pending) - 1
		h := c.pending[last]
		consumed, e := c.manager.DecRefConsumed(h)
		if consumed {
			c.pending = c.pending[:last]
		}
		if e != nil {
			return e
		}
	}
	return nil
}

func (c *rewriteCache) close() error {
	if c == nil || c.closed {
		return nil
	}
	if e := c.reallyFlush(); e != nil {
		return e
	}
	c.entries = nil
	c.pending = nil
	c.closed = true
	return nil
}
