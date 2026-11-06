package rewriter

// Source: src/ast/rewriter/rewriter.cpp:23-204;
// src/ast/rewriter/rewriter.h:31-119; src/ast/ast.h:845-853.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// Concrete source frame. The flags retain the original 1/1/2/2/26-bit fields;
// storing a state/depth/index truncates exactly as the unsigned bitfields do.
// All hot frame and scope records contain handles/scalars, never Go pointers.
type rewriterFrame struct {
	curr        ast.ASTHandle
	flags, spos uint32
}

func (f *rewriterFrame) cacheResult() bool { return f.flags&1 != 0 }
func (f *rewriterFrame) newChild() bool    { return f.flags&2 != 0 }
func (f *rewriterFrame) state() uint32     { return (f.flags >> 2) & 3 }
func (f *rewriterFrame) maxDepth() uint32  { return (f.flags >> 4) & 3 }
func (f *rewriterFrame) index() uint32     { return f.flags >> 6 }
func (f *rewriterFrame) setState(s uint32) { f.flags = (f.flags &^ 12) | ((s & 3) << 2) }
func (f *rewriterFrame) setDepth(d uint32) { f.flags = (f.flags &^ 48) | ((d & 3) << 4) }
func (f *rewriterFrame) setIndex(i uint32) { f.flags = (f.flags & 63) | (i << 6) }

type rewriterScope struct {
	oldRoot     ast.ASTHandle
	oldNumQVars uint32
}

// Cache objects retain their own AST ownership and destructor continuation.
// The directory that owns them is a native Go slice.
func (c *rewriterCore) closeRewriterCaches(caches *[]*ast.ActivityCache) error {
	for i, cache := range *caches {
		if cache != nil {
			if e := cache.Close(); e != nil {
				return e
			}
			c.traceCacheDeallocated(cache)
			(*caches)[i] = nil
		}
	}
	*caches = nil
	return nil
}

type rewriterCore struct {
	self                  *rewriterCore
	manager               *ast.ASTManager
	traceTags             *diagnostic.TraceTags
	proofGen, cancelCheck bool
	cacheStack            []*ast.ActivityCache
	cache                 *ast.ActivityCache
	frames                []rewriterFrame
	results               []ast.ASTHandle
	proofCacheStack       []*ast.ActivityCache
	proofCache            *ast.ActivityCache
	proofResults          []ast.ASTHandle
	root                  ast.ASTHandle
	numQVars              uint32
	scopes                []rewriterScope
	header                uint64
	ready, closed         bool
	cause                 error
}

func newRewriterCore(m *ast.ASTManager, proofGen bool) (*rewriterCore, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(rewriterCore{}).Size())
	if e := m.BudgetField.Reserve(bytes); e != nil {
		return nil, e
	}
	c := &rewriterCore{header: bytes}
	e := c.init(m, proofGen)
	return c, e
}

// Embedded derived objects charge their full header once, then construct this
// base in place with header=0. Standalone construction retains its own charge.
func (c *rewriterCore) init(m *ast.ASTManager, proofGen bool) error {
	c.manager, c.proofGen, c.cancelCheck = m, proofGen, true
	c.self = c
	if e := c.initCacheStack(); e != nil {
		return e
	}
	c.ready = true
	return nil
}
func (c *rewriterCore) open() error {
	if c == nil || c.self != c || c.closed {
		return ast.ErrASTHandle
	}
	if !c.ready {
		return ast.ErrASTDomain
	}
	if c.cause != nil {
		return c.cause
	}
	_, e := c.manager.ProofsEnabled()
	return e
}
func (c *rewriterCore) pushCache(stack *[]*ast.ActivityCache, line int) (*ast.ActivityCache, error) {
	v, e := ast.NewActivityCache(c.manager, nil)
	if e != nil {
		return nil, e
	}
	c.traceCacheAllocated(v, line)
	*stack = append(*stack, v)
	return v, nil
}
func (c *rewriterCore) initCacheStack() error {
	if len(c.cacheStack) != 0 {
		return ast.ErrASTDomain
	}
	v, e := c.pushCache(&c.cacheStack, 25)
	if e != nil {
		return e
	}
	c.cache = v
	if c.proofGen {
		v, e = c.pushCache(&c.proofCacheStack, 29)
		if e != nil {
			return e
		}
		c.proofCache = v
	}
	return nil
}
func (c *rewriterCore) delCacheStack() error {
	if e := c.closeRewriterCaches(&c.cacheStack); e != nil {
		return e
	}
	c.cache = nil
	if c.proofGen {
		if e := c.closeRewriterCaches(&c.proofCacheStack); e != nil {
			return e
		}
		c.proofCache = nil
	}
	return nil
}
func (c *rewriterCore) cacheResult(k, v ast.ASTHandle) error {
	if e := c.open(); e != nil {
		return e
	}
	if ast.ASTSourceDebug && c.proofGen {
		return ast.ErrASTDomain
	}
	if e := c.traceCacheResult(k, v); e != nil {
		return e
	}
	if ast.ASTSourceDebug {
		a, e := c.manager.GetSort(k)
		if e != nil {
			return e
		}
		b, e := c.manager.GetSort(v)
		if e != nil {
			return e
		}
		if a != b {
			return ast.ErrASTDomain
		}
	}
	return c.cache.Insert(k, v)
}
func (c *rewriterCore) cacheResultProof(k, v, pr ast.ASTHandle) error {
	if e := c.open(); e != nil {
		return e
	}
	if e := c.cache.Insert(k, v); e != nil {
		return e
	}
	if !c.proofGen {
		return ast.ErrASTDomain
	}
	return c.proofCache.Insert(k, pr)
}
func (c *rewriterCore) getCached(k ast.ASTHandle) (ast.ASTHandle, error) {
	if e := c.open(); e != nil {
		return ast.ASTHandle{}, e
	}
	return c.cache.Find(k)
}
func (c *rewriterCore) getCachedProof(k ast.ASTHandle) (ast.ASTHandle, error) {
	if e := c.open(); e != nil {
		return ast.ASTHandle{}, e
	}
	if !c.proofGen {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return c.proofCache.Find(k)
}
func (c *rewriterCore) getCacheSize() (uint32, error) {
	if e := c.open(); e != nil {
		return 0, e
	}
	return c.cache.Size()
}
func (c *rewriterCore) resetCache() error {
	if len(c.cacheStack) == 0 {
		return ast.ErrASTDomain
	}
	c.cache = c.cacheStack[0]
	if e := c.cache.Reset(); e != nil {
		return e
	}
	if c.proofGen {
		if len(c.proofCacheStack) == 0 {
			return ast.ErrASTDomain
		}
		c.proofCache = c.proofCacheStack[0]
		if e := c.proofCache.Reset(); e != nil {
			return e
		}
	}
	return nil
}
func (c *rewriterCore) beginScope() (err error) {
	if e := c.open(); e != nil {
		return e
	}
	defer func() {
		if err != nil {
			c.cause = err
		}
	}()
	c.scopes = append(c.scopes, rewriterScope{c.root, c.numQVars})
	level := len(c.scopes)
	if level > len(c.cacheStack) || c.proofGen && len(c.proofCacheStack) != len(c.cacheStack) {
		return ast.ErrASTDomain
	}
	if level == len(c.cacheStack) {
		if _, e := c.pushCache(&c.cacheStack, 99); e != nil {
			return e
		}
		if c.proofGen {
			if _, e := c.pushCache(&c.proofCacheStack, 101); e != nil {
				return e
			}
		}
	}
	c.cache = c.cacheStack[level]
	if e := c.cache.Reset(); e != nil {
		return e
	}
	if c.proofGen {
		c.proofCache = c.proofCacheStack[level]
		if e := c.proofCache.Reset(); e != nil {
			return e
		}
	}
	return nil
}
func (c *rewriterCore) endScope() (err error) {
	if e := c.open(); e != nil {
		return e
	}
	if len(c.scopes) == 0 {
		return ast.ErrASTDomain
	}
	defer func() {
		if err != nil {
			c.cause = err
		}
	}()
	if e := c.cache.Reset(); e != nil {
		return e
	}
	if c.proofGen {
		if e := c.proofCache.Reset(); e != nil {
			return e
		}
	}
	s := c.scopes[len(c.scopes)-1]
	c.root = s.oldRoot
	c.numQVars = s.oldNumQVars
	c.scopes = c.scopes[:len(c.scopes)-1]
	c.cache = c.cacheStack[len(c.scopes)]
	if c.proofGen {
		c.proofCache = c.proofCacheStack[len(c.scopes)]
	}
	return nil
}
func (c *rewriterCore) mustCache(t ast.ASTHandle) (bool, error) {
	if e := c.open(); e != nil {
		return false, e
	}
	refs, e := c.manager.GetRefCount(t)
	if e != nil || refs <= 1 || t == c.root {
		return false, e
	}
	kind, e := c.manager.GetKind(t)
	if e != nil {
		return false, e
	}
	if kind == ast.ASTQuantifier {
		return true, nil
	}
	if kind != ast.ASTApp {
		return false, nil
	}
	n, e := c.manager.GetNumArgs(t)
	return n > 0, e
}
func (c *rewriterCore) pushFrameCore(t ast.ASTHandle, cache bool, state, depth uint32) error {
	if e := c.open(); e != nil {
		return e
	}
	n := uint32(len(c.results))
	if ast.ASTSourceDebug && c.proofGen && len(c.results) != len(c.proofResults) {
		return ast.ErrASTDomain
	}
	fr := rewriterFrame{curr: t, spos: n}
	fr.setState(state)
	fr.setDepth(depth)
	if cache {
		fr.flags |= 1
	}
	c.frames = append(c.frames, fr)
	return nil
}
func (c *rewriterCore) pushFrame(t ast.ASTHandle, state uint32) error {
	cache, e := c.mustCache(t)
	if e != nil {
		return e
	}
	return c.pushFrameCore(t, cache, state, 3)
}
func (c *rewriterCore) isChildOfTopFrame(t ast.ASTHandle) (bool, error) {
	if e := c.open(); e != nil {
		return false, e
	}
	if len(c.frames) == 0 {
		return true, nil
	}
	parent := c.frames[len(c.frames)-1].curr
	m := c.manager
	kind, e := m.GetKind(parent)
	if e != nil {
		return false, e
	}
	switch kind {
	case ast.ASTApp:
		n, e := m.GetNumArgs(parent)
		if e != nil {
			return false, e
		}
		for i := uint32(0); i < n; i++ {
			child, e := m.GetArg(parent, i)
			if e != nil {
				return false, e
			}
			if child == t {
				return true, nil
			}
		}
	case ast.ASTQuantifier:
		child, e := m.GetQuantifierBody(parent)
		if e != nil {
			return false, e
		}
		if child == t {
			return true, nil
		}
		n, e := m.GetNumPatterns(parent)
		if e != nil {
			return false, e
		}
		for i := uint32(0); i < n; i++ {
			child, e := m.GetPattern(parent, i)
			if e != nil {
				return false, e
			}
			if child == t {
				return true, nil
			}
		}
		n, e = m.GetNumNoPatterns(parent)
		if e != nil {
			return false, e
		}
		for i := uint32(0); i < n; i++ {
			child, e := m.GetNoPattern(parent, i)
			if e != nil {
				return false, e
			}
			if child == t {
				return true, nil
			}
		}
	}
	return false, nil
}
func (c *rewriterCore) setNewChild(old ast.ASTHandle) error {
	if e := c.open(); e != nil {
		return e
	}
	if ast.ASTSourceDebug {
		yes, e := c.isChildOfTopFrame(old)
		if e != nil {
			return e
		}
		if !yes {
			return ast.ErrASTDomain
		}
	}
	if len(c.frames) != 0 {
		c.frames[len(c.frames)-1].flags |= 2
	}
	return nil
}
func (c *rewriterCore) setNewChildIfDifferent(old, new ast.ASTHandle) error {
	if old == new {
		return c.open()
	}
	return c.setNewChild(old)
}
func (c *rewriterCore) elimReflexProofs(spos uint32) error {
	if e := c.open(); e != nil {
		return e
	}
	if ast.ASTSourceDebug && !c.proofGen {
		return ast.ErrASTDomain
	}
	if uint64(spos) > uint64(len(c.proofResults)) {
		return ast.ErrASTDomain
	}
	j := int(spos)
	for i := int(spos); i < len(c.proofResults); i++ {
		pr := c.proofResults[i]
		if !pr.IsNull() {
			// Transfer each nonnull reference once; discarded entries are only nulls.
			c.proofResults[j] = pr
			if i != j {
				c.proofResults[i] = ast.ASTHandle{}
			}
			j++
		}
	}
	clear(c.proofResults[j:])
	c.proofResults = c.proofResults[:j]
	return nil
}
func (c *rewriterCore) SetCancelCheck(f bool) error {
	if e := c.open(); e != nil {
		return e
	}
	c.cancelCheck = f
	return nil
}
func (c *rewriterCore) Reset() (err error) {
	if e := c.open(); e != nil {
		return e
	}
	defer func() {
		if err != nil {
			c.cause = err
		}
	}()
	if e := c.resetCache(); e != nil {
		return e
	}
	c.frames = c.frames[:0]
	if e := shrinkRewriterRoots(c.manager, &c.results, 0); e != nil {
		return e
	}
	if c.proofGen {
		if e := shrinkRewriterRoots(c.manager, &c.proofResults, 0); e != nil {
			return e
		}
	}
	c.root = ast.ASTHandle{}
	c.numQVars = 0
	c.scopes = c.scopes[:0]
	return nil
}
func (c *rewriterCore) freeMemory() error {
	if e := c.delCacheStack(); e != nil {
		return e
	}
	c.frames = nil
	if e := shrinkRewriterRoots(c.manager, &c.results, 0); e != nil {
		return e
	}
	c.scopes = nil
	c.results = nil
	// Source deliberately does not finalize the proof result-stack here.
	return nil
}
func (c *rewriterCore) Cleanup() (err error) {
	if e := c.open(); e != nil {
		return e
	}
	defer func() {
		if err != nil {
			c.cause = err
		}
	}()
	if e := c.freeMemory(); e != nil {
		return e
	}
	if e := c.initCacheStack(); e != nil {
		return e
	}
	c.root = ast.ASTHandle{}
	c.numQVars = 0
	return nil
}
func (c *rewriterCore) Close() error {
	if c == nil || c.self != c {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	if e := c.delCacheStack(); e != nil {
		return e
	}
	// Destructor body precedes member destruction in reverse declaration order.
	c.scopes = nil
	if e := shrinkRewriterRoots(c.manager, &c.proofResults, 0); e != nil {
		return e
	}
	c.proofResults = nil
	if e := shrinkRewriterRoots(c.manager, &c.results, 0); e != nil {
		return e
	}
	c.results = nil
	c.frames = nil
	c.manager.BudgetField.Release(c.header)
	c.header = 0
	c.closed = true
	return nil
}
