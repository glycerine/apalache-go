package ast

// Source: src/ast/act_cache.cpp:56-225; src/ast/act_cache.h:23-53.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/insdict"
)

// Native entries own the key/value references recorded by their acquisition
// flags. Insertion order makes stopped reference cleanup deterministic.
type astActivityEntry struct {
	key, value                          ASTHandle
	accessed, dummy, keyHeld, valueHeld bool
}

// ActivityCache's queue borrows keys owned by live entries. Entries remain
// stable while numerical/AST deletion may stop and be acknowledged later.
type ActivityCache struct {
	self                     *ActivityCache
	manager                  *ASTManager
	entries                  insdict.Dict[ASTHandle, *astActivityEntry]
	queue                    []ASTHandle
	qhead, unused, maxUnused uint32
	header                   uint64
	retiring                 astActivityEntry
	incoming                 ASTHandle
	incomingHeld             bool
	pending                  bool
	cause                    error
	closed                   bool
}

func activityHandleHash(h ASTHandle) uint64 {
	return h.origin ^ uint64(h.slot)*0x9e3779b97f4a7c15 ^ uint64(h.generation)*0xbf58476d1ce4e5b9
}

func NewActivityCache(m *ASTManager, maxUnused *uint32) (*ActivityCache, error) {
	if m == nil {
		return nil, ErrASTHandle
	}
	if e := m.lock(); e != nil {
		return nil, e
	}
	n := m.table.size
	m.mu.Unlock()
	if maxUnused != nil {
		n = *maxUnused
	}
	if n < 1024 {
		n = 1024
	}
	bytes := uint64(reflect.TypeOf(ActivityCache{}).Size())
	if e := m.BudgetField.Reserve(bytes); e != nil {
		return nil, e
	}
	c := &ActivityCache{manager: m, maxUnused: n, header: bytes}
	c.entries = *insdict.NewDictFunc[ASTHandle, *astActivityEntry](activityHandleHash)
	c.self = c
	return c, nil
}
func (c *ActivityCache) open() error {
	if c == nil || c.self != c || c.closed {
		return ErrASTHandle
	}
	if c.cause != nil {
		return c.cause
	}
	_, e := c.manager.ProofsEnabled()
	return e
}
func (c *ActivityCache) dec(h ASTHandle) (bool, error) {
	m := c.manager
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	if h.IsNull() {
		return true, nil
	}
	r, e := m.internal(h)
	if e != nil {
		return false, e
	}
	return m.decNode(r)
}
func (c *ActivityCache) retire(entry *astActivityEntry) error {
	if entry.keyHeld {
		consumed, e := c.dec(entry.key)
		if consumed {
			entry.keyHeld = false
		}
		if e != nil {
			return e
		}
	}
	if entry.valueHeld {
		consumed, e := c.dec(entry.value)
		if consumed {
			entry.valueHeld = false
		}
		if e != nil {
			return e
		}
	}
	return nil
}
func (c *ActivityCache) finishPending() error {
	if !c.pending {
		return nil
	}
	if e := c.retire(&c.retiring); e != nil {
		return e
	}
	c.pending = false
	c.retiring = astActivityEntry{}
	return nil
}
func (c *ActivityCache) deleteUnused() error {
	size := uint32(len(c.queue))
	for c.qhead < size {
		k := c.queue[c.qhead]
		c.qhead++
		entry, found := c.entries.Get2(k)
		if !found {
			return ErrASTDomain
		}
		if !entry.accessed {
			c.unused--
			c.retiring = *entry
			c.pending = true
			c.entries.DelPackMaybe(k)
			if e := c.finishPending(); e != nil {
				return e
			}
			break
		}
	}
	if c.qhead == size {
		c.queue = c.queue[:0]
		c.qhead = 0
	} else if c.qhead > c.maxUnused {
		copy(c.queue, c.queue[c.qhead:])
		c.queue = c.queue[:size-c.qhead]
		c.qhead = 0
	}
	return nil
}
func (c *ActivityCache) Insert(k, v ASTHandle) (err error) {
	if e := c.open(); e != nil {
		return e
	}
	defer func() {
		if err != nil {
			c.cause = err
		}
	}()
	if k.IsNull() {
		return ErrASTDomain
	}
	// Checked handles replace invalid source pointers, without changing the
	// statement order for valid expressions.
	m := c.manager
	if _, e := m.GetSort(k); e != nil {
		return e
	}
	if !v.IsNull() {
		if _, e := m.GetSort(v); e != nil {
			return e
		}
	}
	if c.unused >= c.maxUnused {
		if e := c.deleteUnused(); e != nil {
			return e
		}
	}
	entry, found := c.entries.Get2(k)
	if !found {
		entry = &astActivityEntry{key: k, dummy: true}
		c.entries.Put(k, entry)
	}
	if entry.dummy {
		if e := m.IncRef(k); e != nil {
			return e
		}
		entry.keyHeld = true
		if e := m.IncRef(v); e != nil {
			return e
		}
		entry.valueHeld = !v.IsNull()
		entry.value = v
		entry.dummy = false
		if uint64(len(c.queue)) >= uint64(^uint32(0)) {
			return arena.ErrBounds
		}
		c.queue = append(c.queue, k)
		c.unused++
	} else if entry.value != v {
		if e := m.IncRef(v); e != nil {
			return e
		}
		// Publish after the old-value DEC, preserving source statement order.
		c.incoming, c.incomingHeld = v, !v.IsNull()
		consumed, e := c.dec(entry.value)
		if consumed {
			entry.valueHeld = false
		}
		if e != nil {
			return e
		}
		entry.value, entry.valueHeld = v, c.incomingHeld
		c.incoming, c.incomingHeld = ASTHandle{}, false
		entry.accessed = false
		// Preserve the source quirk: replacing an accessed entry does not increment
		// unused and does not append its key to the queue.
	}
	return nil
}
func (c *ActivityCache) Find(k ASTHandle) (ASTHandle, error) {
	if e := c.open(); e != nil {
		return ASTHandle{}, e
	}
	if k.IsNull() {
		return ASTHandle{}, ErrASTDomain
	}
	if _, e := c.manager.GetHash(k); e != nil {
		return ASTHandle{}, e
	}
	entry, found := c.entries.Get2(k)
	if !found {
		return ASTHandle{}, nil
	}
	if !entry.accessed {
		entry.accessed = true
		if ASTSourceDebug && c.unused == 0 {
			return ASTHandle{}, ErrASTDomain
		}
		c.unused--
	}
	return entry.value, nil
}
func (c *ActivityCache) decRefs() error {
	if c.incomingHeld {
		consumed, e := c.dec(c.incoming)
		if consumed {
			c.incomingHeld = false
			c.incoming = ASTHandle{}
		}
		if e != nil {
			return e
		}
	}
	if e := c.finishPending(); e != nil {
		return e
	}
	for _, entry := range c.entries.All() {
		if e := c.retire(entry); e != nil {
			return e
		}
	}
	return nil
}
func (c *ActivityCache) clear(cleanup bool) (err error) {
	if c == nil || c.self != c || c.closed {
		return ErrASTHandle
	}
	defer func() {
		if err != nil {
			c.cause = err
		}
	}()
	if e := c.decRefs(); e != nil {
		return e
	}
	c.entries.Clear()
	c.queue = c.queue[:0]
	if cleanup {
		c.queue = nil
	}
	c.unused = 0
	c.qhead = 0
	c.cause = nil
	return nil
}
func (c *ActivityCache) Reset() error         { return c.clear(false) }
func (c *ActivityCache) Cleanup() error       { return c.clear(true) }
func (c *ActivityCache) CheckInvariant() bool { return true }
func (c *ActivityCache) Size() (uint32, error) {
	if e := c.open(); e != nil {
		return 0, e
	}
	return uint32(c.entries.Len()), nil
}
func (c *ActivityCache) Empty() (bool, error) {
	n, e := c.Size()
	return n == 0, e
}
func (c *ActivityCache) Close() error {
	if c == nil || c.self != c {
		return ErrASTHandle
	}
	if c.closed {
		return nil
	}
	if e := c.decRefs(); e != nil {
		return e
	}
	c.entries = insdict.Dict[ASTHandle, *astActivityEntry]{}
	c.manager.BudgetField.Release(c.header)
	c.queue = nil
	c.header = 0
	c.closed = true
	return nil
}
