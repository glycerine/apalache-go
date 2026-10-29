package api

// Source: src/api/api_context.cpp:35-55,109-119,215-246;
// src/api/api_context.h:78-84; src/api/api_util.h:37-49; src/util/ref.h:79-85.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/insdict"
)

var errAPIObjectOwner = errors.New("api: invalid, copied or retired context result/object owner")

// This is the result/object and selected sort-check portion of api::context,
// borrowing the real AST manager. It is not a replacement for the unported context constructor,
// which must also own its services, plugin utilities and tactic registry.
// The insertion-ordered dictionary gives deterministic teardown without
// reproducing the source hash table's physical buckets. Values index a cold
// directory of API wrappers.
type apiContextResults struct {
	self                          *apiContextResults
	manager                       *ast.ASTManager
	lastResult, astTrail          []ast.ASTHandle
	lastObject, pendingDestroy    *apiObject
	allocated                     insdict.Dict[uint32, uint32]
	objects                       []*apiObject
	freeIDs                       []uint32
	ownerBytes                    uint64
	trace                         *diagnostic.TraceTags
	warnings                      *diagnostic.WarningService
	errorHandler                  func(*apiContextResults, uint32)
	memoryProcess                 *memory.ProcessAccounting
	pendingDestroyLine            int
	errorCode                     uint32
	userRefCount, closing, closed bool
}

// One cold virtual destructor per API wrapper translates api::object's virtual
// destructor. Derived goals/models/solvers retain their own source AST roots.
type apiObjectDestructor interface{ apiDestroy() error }
type apiObject struct {
	self           *apiObject
	context        *apiContextResults
	destructor     apiObjectDestructor
	id, references uint32
	ownerBytes     uint64
	closed         bool
}

func newAPIContextResults(m *ast.ASTManager, rc bool) (c *apiContextResults, err error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err = m.ProofsEnabled(); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(apiContextResults{}).Size())
	if err = m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	c = &apiContextResults{manager: m, userRefCount: rc, ownerBytes: n, trace: diagnostic.BorrowProcessTraceTags(), warnings: diagnostic.BorrowProcessWarningService()}
	c.self = c
	if q := m.NumeralService(); q != nil {
		if runtime := q.RuntimeOwner(); runtime != nil {
			c.memoryProcess = runtime.ProcessOwner()
		}
	}
	return c, nil
}

func (c *apiContextResults) open() error {
	if c == nil || c.self != c {
		return errAPIObjectOwner
	}
	if c.closed {
		return arena.ErrClosed
	}
	if c.closing {
		return ast.ErrASTPending
	}
	return nil
}

// Native result slices own one AST reference per entry, including duplicates.
func (c *apiContextResults) appendASTRoot(roots *[]ast.ASTHandle, h ast.ASTHandle) error {
	if e := c.manager.IncRef(h); e != nil {
		return e
	}
	*roots = append(*roots, h)
	return nil
}

// Keep only unconsumed roots after an interrupted callback. A consumed DEC may
// still report a later destructor error; retry must not release that root twice.
func (c *apiContextResults) resetASTRoots(roots *[]ast.ASTHandle) error {
	values := *roots
	for i, h := range values {
		consumed, e := c.manager.DecRefConsumed(h)
		next := i
		if consumed {
			values[i] = ast.ASTHandle{}
			next++
		}
		if e != nil {
			*roots = values[next:]
			return e
		}
		if !consumed {
			*roots = values[next:]
			return errAPIObjectOwner
		}
	}
	*roots = values[:0]
	return nil
}

func (c *apiContextResults) saveASTTrail(h ast.ASTHandle) (err error) {
	if e := c.open(); e != nil {
		return e
	}
	if diagnostic.DebugBuild {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return e
		}
		if enabled {
			contained := !h.IsNull()
			if contained {
				_, e = c.manager.GetKind(h)
				contained = e == nil
			}
			if !contained {
				if e = diagnostic.DebugAssertionFailure(d, "src/api/api_context.cpp", 216, "m().contains(n)"); e != nil {
					return e
				}
			}
		}
	}
	if !c.userRefCount {
		return c.appendASTRoot(&c.astTrail, h)
	}
	// Keep an alias alive across reset, exactly as context.cpp:220-225.
	if e := c.manager.IncRef(h); e != nil {
		return e
	}
	defer func() { err = errors.Join(err, c.manager.DecRef(h)) }()
	if e := c.resetASTRoots(&c.lastResult); e != nil {
		return e
	}
	return c.appendASTRoot(&c.lastResult, h)
}

func (c *apiContextResults) saveMultipleASTTrail(h ast.ASTHandle) error {
	if e := c.open(); e != nil {
		return e
	}
	if c.userRefCount {
		return c.appendASTRoot(&c.lastResult, h)
	}
	return c.appendASTRoot(&c.astTrail, h)
}

func (c *apiContextResults) resetLastResult() error {
	if e := c.open(); e != nil {
		return e
	}
	if c.userRefCount {
		if e := c.resetASTRoots(&c.lastResult); e != nil {
			return e
		}
	}
	return c.assignLastObject(nil)
}

func (c *apiContextResults) saveObject(o *apiObject) error {
	if e := c.open(); e != nil {
		return e
	}
	return c.assignLastObject(o)
}

func (c *apiContextResults) assignLastObject(o *apiObject) error {
	if o != nil {
		if o.context != c {
			return errAPIObjectOwner
		}
		if e := o.incRef(); e != nil {
			return e
		}
	}
	if c.lastObject != nil {
		if e := c.lastObject.decRef(); e != nil {
			return e
		}
	}
	c.lastObject = o
	return nil
}

func (c *apiContextResults) growObjectDirectory(id uint32) error {
	if uint64(id) < uint64(len(c.objects)) {
		return nil
	}
	if uint64(id)+1 > uint64(int(^uint(0)>>1)) {
		return arena.ErrBounds
	}
	newLen := int(id) + 1
	if newLen > cap(c.objects) {
		v := make([]*apiObject, newLen)
		copy(v, c.objects)
		c.objects = v
	} else {
		c.objects = c.objects[:newLen]
	}
	return nil
}

func (c *apiContextResults) addObject(o *apiObject) (uint32, error) {
	if e := c.open(); e != nil {
		return 0, e
	}
	id := uint32(c.allocated.Len())
	if n := len(c.freeIDs); n != 0 {
		id = c.freeIDs[n-1]
		c.freeIDs = c.freeIDs[:n-1]
	}
	if e := c.growObjectDirectory(id); e != nil {
		return 0, e
	}
	c.allocated.Put(id, id)
	c.objects[id] = o
	return id, nil
}

func (c *apiContextResults) pushFreeID(id uint32) error {
	c.freeIDs = append(c.freeIDs, id)
	return nil
}

func (c *apiContextResults) delObject(o *apiObject) error {
	// Source publication order is free-list push, table removal, deallocation.
	if e := c.pushFreeID(o.id); e != nil {
		return e
	}
	c.pendingDestroy = o
	c.pendingDestroyLine = 54
	c.allocated.Del(o.id)
	return c.destroyObject(o, 54)
}

func (c *apiContextResults) destroyObject(o *apiObject, line int) error {
	c.pendingDestroy = o
	c.pendingDestroyLine = line
	if e := o.destroy(); e != nil {
		c.pendingDestroy = o
		return e
	}
	if a, ok := o.destructor.(*apiASTMapObject); ok {
		c.traceASTMapDeallocated(a, line)
	}
	if g, ok := o.destructor.(*apiGoalObject); ok {
		c.traceGoalDeallocated(g, line)
	}
	c.objects[o.id] = nil
	c.pendingDestroy = nil
	c.pendingDestroyLine = 0
	return nil
}

func (o *apiObject) open() error {
	if o == nil || o.self != o || o.closed || o.context == nil || o.context.self != o.context || o.context.closed {
		return errAPIObjectOwner
	}
	_, allocated := o.context.allocated.Get2(o.id)
	if uint64(o.id) >= uint64(len(o.context.objects)) || o.context.objects[o.id] != o || !allocated {
		return errAPIObjectOwner
	}
	return nil
}
func (o *apiObject) incRef() error {
	if e := o.open(); e != nil {
		return e
	}
	o.references++
	return nil
}
func (o *apiObject) decRef() error {
	if e := o.open(); e != nil {
		return e
	}
	if diagnostic.DebugBuild {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return e
		}
		if enabled && o.references == 0 {
			if e = diagnostic.DebugAssertionFailure(d, "src/api/api_context.cpp", 39, "m_ref_count > 0"); e != nil {
				return e
			}
		}
	}
	if o.references == 0 {
		return errAPIObjectOwner
	}
	o.references--
	if o.references == 0 {
		return o.context.delObject(o)
	}
	return nil
}
func (o *apiObject) destroy() error {
	if o.closed {
		return nil
	}
	if e := o.destructor.apiDestroy(); e != nil {
		return e
	}
	o.closed = true
	o.context.manager.Budget().Release(o.ownerBytes)
	o.ownerBytes = 0
	return nil
}

func (c *apiContextResults) Close() error {
	if c == nil || c.self != c {
		return errAPIObjectOwner
	}
	if c.closed {
		return nil
	}
	c.closing = true
	if c.pendingDestroy != nil {
		if e := c.destroyObject(c.pendingDestroy, c.pendingDestroyLine); e != nil {
			return e
		}
	}
	// A returning Go failure may have consumed the source DEC before its
	// free-list push or derived destructor refused. Owning teardown drains that
	// zero-ref wrapper from the registry rather than repeating the DEC.
	if c.lastObject != nil && (c.lastObject.closed || c.lastObject.references == 0) {
		c.lastObject = nil
	}
	if e := c.assignLastObject(nil); e != nil {
		return e
	}
	// Restart begin after each removal: a derived destructor may release other
	// registered objects, and the source hash table can compact during removal.
	for c.allocated.Len() != 0 {
		var id uint32
		for key := range c.allocated.All() {
			id = key
			break
		}
		o := c.objects[id]
		c.pendingDestroy = o
		if diagnostic.DebugBuild {
			// Source typeid(*val).name() is compiler-specific; the Go concrete
			// destructor type identifies the same registered API object.
			_ = diagnostic.BorrowProcessWarningService().Warningf("Uncollected memory: %d: %s", id, reflect.TypeOf(o.destructor).String())
		}
		c.allocated.Del(id)
		if err := c.destroyObject(o, 116); err != nil {
			return err
		}
	}
	c.freeIDs = nil
	c.allocated.Clear()
	c.objects = nil
	if e := c.resetASTRoots(&c.astTrail); e != nil {
		return e
	}
	c.astTrail = nil
	if e := c.resetASTRoots(&c.lastResult); e != nil {
		return e
	}
	c.lastResult = nil
	c.manager.Budget().Release(c.ownerBytes)
	c.ownerBytes = 0
	c.closed = true
	return nil
}
