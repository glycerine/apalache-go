package rewriter

// Source: src/ast/rewriter/bv_trailing.cpp:24-244,338-405;
// src/ast/rewriter/bv_trailing.h:24-44.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

const bvTrailingDepth uint32 = 5

// BVTrailing borrows the extraction procedure/manager and owns five lazy
// count caches and reached local frames. Its Go header is charged once;
// do not copy it.
type BVTrailing struct {
	self                          *BVTrailing
	extract                       *MkExtractProc
	manager                       *ast.ASTManager
	tables                        [6]*bvTrailingTable
	header                        uint64
	running, closed, dtorTraced   bool
	failure                       error
	frames                        *bvTrailingFrame
	countCall, removeCall, eqCall bvTrailingCall
}

func NewBVTrailing(extract *MkExtractProc) (*BVTrailing, error) {
	m, e := rotationManager(extract)
	if e != nil {
		return nil, e
	}
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	header := uint64(reflect.TypeOf(BVTrailing{}).Size())
	if e = m.Budget().Reserve(header); e != nil {
		return nil, e
	}
	p := &BVTrailing{extract: extract, manager: m, header: header}
	p.self = p
	p.traceImpAllocated()
	p.traceLifecycle(false)
	return p, nil
}

func (p *BVTrailing) enter() error {
	if p == nil || p.self != p || p.closed {
		return ast.ErrASTHandle
	}
	if p.running || p.failure != nil {
		return ast.ErrASTPending
	}
	if p.extract == nil || p.extract.self != p.extract || p.extract.closed {
		return ast.ErrASTHandle
	}
	if _, e := p.manager.ProofsEnabled(); e != nil {
		return e
	}
	p.running = true
	return nil
}
func (p *BVTrailing) leave(body error) {
	p.running = false
	if body != nil {
		p.failure = body
	}
}

func (p *BVTrailing) Count(arg ast.ASTHandle) (min, max uint32, body error) {
	if e := p.enter(); e != nil {
		return 0, 0, e
	}
	defer func() { p.leave(body) }()
	return p.count(arg, bvTrailingDepth)
}

func (p *BVTrailing) count(arg ast.ASTHandle, depth uint32) (min, max uint32, body error) {
	return p.countTracked(arg, depth, &p.countCall)
}
func (p *BVTrailing) countTracked(arg ast.ASTHandle, depth uint32, call *bvTrailingCall) (min, max uint32, body error) {
	m := p.manager
	if ast.ASTSourceDebug {
		yes, e := m.IsBitVector(arg)
		if e != nil {
			return 0, 0, e
		}
		if !yes {
			return 0, 0, ast.ErrASTDomain
		}
	}
	if depth == 0 {
		max, body = m.BitVectorSize(arg)
		return 0, max, body
	}
	if depth > bvTrailingDepth {
		return 0, 0, ast.ErrASTDomain
	}
	if table := p.tables[depth]; table != nil {
		entry, found, e := table.find(m, arg)
		if e != nil {
			return 0, 0, e
		}
		if found {
			if e = p.traceCacheEntry(false, depth, arg, entry.min, entry.max); e != nil {
				return 0, 0, e
			}
			return entry.min, entry.max, nil
		}
	}
	min, max, body = p.countCore(arg, depth, call)
	if body != nil {
		return
	}
	if body = p.traceCount(arg, min, max); body != nil {
		return
	}
	if ast.ASTSourceDebug {
		width, e := m.BitVectorSize(arg)
		if e != nil {
			return min, max, e
		}
		if min > max || max > width {
			return min, max, ast.ErrASTDomain
		}
	}
	body = p.cache(depth, arg, min, max)
	return
}

func (p *BVTrailing) cache(depth uint32, arg ast.ASTHandle, min, max uint32) error {
	if depth == 0 {
		return nil
	}
	if p.tables[depth] == nil {
		table, e := newBVTrailingTable(p.manager)
		if e != nil {
			return e
		}
		p.traceTableAllocated(table)
		p.tables[depth] = table
	}
	if ast.ASTSourceDebug {
		_, found, e := p.tables[depth].find(p.manager, arg)
		if e != nil {
			return e
		}
		if found {
			return ast.ErrASTDomain
		}
	}
	if e := p.manager.IncRef(arg); e != nil {
		return e
	}
	// Native insertion publishes the acquired reference without a fallible
	// container-growth step between INC and transfer to the entry.
	p.tables[depth].insert(arg, min, max)
	return p.traceCacheEntry(true, depth, arg, min, max)
}

func (p *BVTrailing) ResetCache(condition uint32) (body error) {
	if e := p.enter(); e != nil {
		return e
	}
	defer func() { p.leave(body) }()
	return p.resetCache(condition)
}
func (p *BVTrailing) resetCache(condition uint32) error {
	if ast.ASTSourceDebug && p.tables[0] != nil {
		return ast.ErrASTDomain
	}
	for depth := uint32(1); depth <= bvTrailingDepth; depth++ {
		table := p.tables[depth]
		if table == nil {
			continue
		}
		p.traceCacheReset(depth, condition, false)
		if condition != 0 && uint32(table.entries.Len()) < condition {
			continue
		}
		p.traceCacheReset(depth, condition, true)
		if e := table.close(p.manager); e != nil {
			return e
		}
		p.traceTableDeallocated(table)
		p.tables[depth] = nil
	}
	return nil
}

// Close is owning cleanup. It records DEC consumption independently of the
// error so a later child-deletion refusal never decrements the same key twice.
func (p *BVTrailing) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if p.running {
		return ast.ErrASTPending
	}
	for p.frames != nil {
		f := p.frames
		if e := f.destroy(true); e != nil {
			return e
		}
		p.frames = f.next
		p.manager.Budget().Release(f.header)
		f.header = 0
		f.retired = true
	}
	for _, call := range []*bvTrailingCall{&p.removeCall, &p.eqCall, &p.countCall} {
		if e := call.cleanup(p.manager); e != nil {
			return e
		}
	}
	if !p.dtorTraced {
		p.dtorTraced = true
		p.traceLifecycle(true)
	}
	if e := p.resetCache(0); e != nil {
		return e
	}
	p.manager.Budget().Release(p.header)
	p.header = 0
	p.traceImpDeallocated()
	p.closed = true
	return nil
}
