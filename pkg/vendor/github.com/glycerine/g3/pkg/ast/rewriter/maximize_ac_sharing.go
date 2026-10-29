package rewriter

// Source: src/ast/rewriter/maximize_ac_sharing.cpp:24-171;
// src/ast/rewriter/maximize_ac_sharing.h:24-122.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

const maximizeACArgumentLimit = 128

type acSharingKey struct {
	decl, first, second ast.ASTHandle
}

type acSharingEntry struct {
	key       acSharingKey
	firstRef  ast.ASTRef
	secondRef ast.ASTRef
}

// MaximizeBVSharingConfiguration is the source maximize_bv_sharing reducer.
// Pair entries retain their operands until their scope is popped or the owner
// closes; reset only clears the opportunistic lookup table.
type MaximizeBVSharingConfiguration struct {
	DefaultConfiguration
	self        *MaximizeBVSharingConfiguration
	manager     *ast.ASTManager
	kinds       []int32
	entries     []acSharingEntry
	cache       map[acSharingKey]struct{}
	scopes      []int
	initialized bool
	closed      bool
}

func NewMaximizeBVSharingConfiguration(m *ast.ASTManager) (*MaximizeBVSharingConfiguration, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	c := &MaximizeBVSharingConfiguration{manager: m, cache: make(map[acSharingKey]struct{})}
	c.self = c
	c.defaults.manager = m
	return c, nil
}

func (c *MaximizeBVSharingConfiguration) open() error {
	if c == nil || c.self != c || c.closed || c.manager == nil {
		return ast.ErrASTHandle
	}
	_, e := c.manager.ProofsEnabled()
	return e
}

func (c *MaximizeBVSharingConfiguration) init() {
	if c.initialized {
		return
	}
	// OP_BADD, OP_BMUL, OP_BOR, OP_BAND.
	c.kinds = append(c.kinds, 4, 6, 30, 31)
	c.initialized = true
}

func (c *MaximizeBVSharingConfiguration) canonical(decl, a, b ast.ASTHandle) (acSharingKey, error) {
	aID, e := c.manager.GetASTID(a)
	if e != nil {
		return acSharingKey{}, e
	}
	bID, e := c.manager.GetASTID(b)
	if e != nil {
		return acSharingKey{}, e
	}
	if aID > bID {
		a, b = b, a
	}
	return acSharingKey{decl: decl, first: a, second: b}, nil
}

func (c *MaximizeBVSharingConfiguration) contains(decl, a, b ast.ASTHandle) (bool, error) {
	k, e := c.canonical(decl, a, b)
	if e != nil {
		return false, e
	}
	_, yes := c.cache[k]
	return yes, nil
}

func (c *MaximizeBVSharingConfiguration) insert(decl, a, b ast.ASTHandle) (result error) {
	k, e := c.canonical(decl, a, b)
	if e != nil {
		return e
	}
	entry := acSharingEntry{key: k}
	if entry.firstRef, e = c.manager.NewRef(k.first); e != nil {
		return e
	}
	if entry.secondRef, e = c.manager.NewRef(k.second); e != nil {
		cleanup := c.manager.DelRef(&entry.firstRef)
		if cleanup != nil {
			c.entries = append(c.entries, entry)
		}
		return errors.Join(e, cleanup)
	}
	c.entries = append(c.entries, entry)
	c.cache[k] = struct{}{}
	return nil
}

func (c *MaximizeBVSharingConfiguration) restoreEntries(limit int) error {
	if limit < 0 || limit > len(c.entries) {
		return ast.ErrASTDomain
	}
	for len(c.entries) > limit {
		i := len(c.entries) - 1
		e := &c.entries[i]
		delete(c.cache, e.key)
		if e.secondRef != (ast.ASTRef{}) {
			if x := c.manager.DelRef(&e.secondRef); x != nil {
				return x
			}
		}
		if e.firstRef != (ast.ASTRef{}) {
			if x := c.manager.DelRef(&e.firstRef); x != nil {
				return x
			}
		}
		c.entries = c.entries[:i]
	}
	return nil
}

func (c *MaximizeBVSharingConfiguration) PushScope() error {
	if e := c.open(); e != nil {
		return e
	}
	c.init()
	c.scopes = append(c.scopes, len(c.entries))
	return nil
}

func (c *MaximizeBVSharingConfiguration) PopScope(count uint32) error {
	if e := c.open(); e != nil {
		return e
	}
	if count == 0 || uint64(count) > uint64(len(c.scopes)) {
		return ast.ErrASTDomain
	}
	level := len(c.scopes) - int(count)
	if e := c.restoreEntries(c.scopes[level]); e != nil {
		return e
	}
	c.scopes = c.scopes[:level]
	return nil
}

func (c *MaximizeBVSharingConfiguration) Reset() {
	if c == nil || c.self != c || c.closed {
		return
	}
	clear(c.cache)
}

func (c *MaximizeBVSharingConfiguration) ReduceApp(decl ast.ASTHandle, arguments ast.AppArguments, out, _ ast.ASTRef) (RewriteStatus, error) {
	if e := c.open(); e != nil {
		return RewriteFailed, e
	}
	associative, e := c.manager.RewriterAssociative(decl)
	if e != nil || !associative || arguments.Count() <= 2 {
		return RewriteFailed, e
	}
	kind, e := c.manager.GetDeclKind(decl)
	if e != nil {
		return RewriteFailed, e
	}
	registered := false
	for _, candidate := range c.kinds {
		if kind == candidate {
			registered = true
			break
		}
	}
	if !registered {
		return RewriteFailed, nil
	}
	args := make([]ast.ASTHandle, arguments.Count())
	for i := range args {
		args[i], e = arguments.At(uint32(i))
		if e != nil {
			return RewriteFailed, e
		}
	}
	var numeral ast.ASTHandle
	if yes, x := c.manager.IsBitVectorNumeral(args[0]); x != nil {
		return RewriteFailed, x
	} else if yes {
		numeral = args[0]
		args = args[1:]
	}
	for len(args) > 1 && len(args) < maximizeACArgumentLimit {
		found := false
		for i := 0; i+1 < len(args) && !found; i++ {
			for j := i + 1; j < len(args); j++ {
				yes, x := c.contains(decl, args[i], args[j])
				if x != nil {
					return RewriteFailed, x
				}
				if !yes {
					continue
				}
				args[i], x = c.manager.MkApp(decl, []ast.ASTHandle{args[i], args[j]})
				if x != nil {
					return RewriteFailed, x
				}
				copy(args[j:], args[j+1:])
				args = args[:len(args)-1]
				found = true
				break
			}
		}
		if !found {
			break
		}
	}
	for len(args) > 1 {
		write := 0
		for i := 0; i < len(args); i += 2 {
			if i == len(args)-1 {
				args[write] = args[i]
			} else {
				if e = c.insert(decl, args[i], args[i+1]); e != nil {
					return RewriteFailed, e
				}
				args[write], e = c.manager.MkApp(decl, []ast.ASTHandle{args[i], args[i+1]})
				if e != nil {
					return RewriteFailed, e
				}
			}
			write++
		}
		args = args[:write]
	}
	result := args[0]
	if !numeral.IsNull() {
		result, e = c.manager.MkApp(decl, []ast.ASTHandle{numeral, result})
		if e != nil {
			return RewriteFailed, e
		}
	}
	return RewriteDone, c.manager.SetRef(out, result)
}

func (c *MaximizeBVSharingConfiguration) Close() error {
	if c == nil || c.self != c {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	if e := c.restoreEntries(0); e != nil {
		return e
	}
	c.closed = true
	return nil
}

// MaximizeBVSharing owns the source rewriter wrapper and scoped configuration.
type MaximizeBVSharing struct {
	self      *MaximizeBVSharing
	manager   *ast.ASTManager
	config    *MaximizeBVSharingConfiguration
	traversal *Traversal
	header    uint64
	closed    bool
}

func NewMaximizeBVSharing(m *ast.ASTManager) (*MaximizeBVSharing, error) {
	c, e := NewMaximizeBVSharingConfiguration(m)
	if e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(MaximizeBVSharing{}).Size() + reflect.TypeOf(MaximizeBVSharingConfiguration{}).Size())
	if e = m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	r := &MaximizeBVSharing{manager: m, config: c, header: bytes}
	r.self = r
	r.traversal, e = NewTraversal(m, c)
	return r, e
}

func (r *MaximizeBVSharing) open() error {
	if r == nil || r.self != r || r.closed || r.config == nil || r.traversal == nil {
		return ast.ErrASTHandle
	}
	return r.config.open()
}

func (r *MaximizeBVSharing) Rewrite(root ast.ASTHandle, out, proof ast.ASTRef) error {
	if e := r.open(); e != nil {
		return e
	}
	if e := r.traversal.Rewrite(root, out); e != nil {
		return e
	}
	proofs, e := r.manager.ProofsEnabled()
	if e != nil || !proofs {
		return e
	}
	result, e := r.manager.RefGet(out)
	if e != nil {
		return e
	}
	var p ast.ASTHandle
	if result == root {
		p, e = r.manager.MkProofReflexivity(root)
	} else {
		p, e = r.manager.MkProofRewrite(root, result)
	}
	if e != nil {
		return e
	}
	return r.manager.SetRef(proof, p)
}

func (r *MaximizeBVSharing) PushScope() error {
	if e := r.open(); e != nil {
		return e
	}
	return r.config.PushScope()
}

func (r *MaximizeBVSharing) PopScope(count uint32) error {
	if e := r.open(); e != nil {
		return e
	}
	return r.config.PopScope(count)
}

func (r *MaximizeBVSharing) Reset() error {
	if e := r.open(); e != nil {
		return e
	}
	r.config.Reset()
	return nil
}

func (r *MaximizeBVSharing) Close() error {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	if r.traversal != nil {
		if e := r.traversal.Close(); e != nil {
			return e
		}
	}
	if r.config != nil {
		if e := r.config.Close(); e != nil {
			return e
		}
	}
	r.manager.Budget().Release(r.header)
	r.header = 0
	r.closed = true
	return nil
}
