package rewriter

// Source: src/ast/rewriter/mk_extract_proc.cpp:19-52;
// src/ast/rewriter/mk_extract_proc.h:22-35.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

// MkExtractProc owns one cached extraction declaration and borrows its manager.
// The declaration owns the cached domain sort; this helper does not INC it.
type MkExtractProc struct {
	self                      *MkExtractProc
	manager                   *ast.ASTManager
	domain, cached            ast.ASTHandle
	high, low                 uint32
	header                    uint64
	acquired, running, closed bool
	failure                   error
}

func NewMkExtractProc(m *ast.ASTManager) (*MkExtractProc, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(MkExtractProc{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	p := &MkExtractProc{header: bytes}
	p.constructSource(m)
	return p, nil
}

// The containing derived rewriter charges this embedded member in its header.
// Standalone construction delegates the same original initialization.
func (p *MkExtractProc) constructSource(m *ast.ASTManager) {
	p.manager = m
	p.low = ^uint32(0)
	p.self = p
}

func (p *MkExtractProc) Apply(high, low uint32, arg ast.ASTHandle) (_ ast.ASTHandle, body error) {
	if p == nil || p.self != p || p.closed {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	if p.running || p.failure != nil {
		return ast.ASTHandle{}, ast.ErrASTPending
	}
	p.running = true
	defer func() {
		p.running = false
		if body != nil {
			p.failure = body
		}
	}()
	m := p.manager
	sort, e := m.GetSort(arg)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	if p.low == low && p.high == high && p.domain == sort {
		return m.MkApp(p.cached, []ast.ASTHandle{arg})
	}
	if !p.cached.IsNull() {
		consumed, e := m.DecRefConsumed(p.cached)
		if consumed {
			p.acquired = false
		}
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	r, e := m.MkBitVectorExtract(high, low, arg)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	p.high = high
	p.low = low
	p.domain = sort
	p.cached, e = m.GetDecl(r)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	if e = m.IncRef(p.cached); e != nil {
		return ast.ASTHandle{}, e
	}
	p.acquired = true
	return r, nil
}

// Close is the checked owning closure: after a refused replacement, an old
// cached pointer whose reference was already consumed must not be decremented.
func (p *MkExtractProc) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if p.running {
		return ast.ErrASTPending
	}
	if p.acquired {
		consumed, e := p.manager.DecRefConsumed(p.cached)
		if consumed {
			p.acquired = false
		}
		if e != nil {
			return e
		}
	}
	p.cached = ast.ASTHandle{}
	p.domain = ast.ASTHandle{}
	p.manager.Budget().Release(p.header)
	p.header = 0
	p.closed = true
	return nil
}
