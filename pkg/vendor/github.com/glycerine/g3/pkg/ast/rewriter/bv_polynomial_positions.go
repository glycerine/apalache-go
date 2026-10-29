package rewriter

// Source: src/ast/rewriter/poly_rewriter.h:32;
// src/ast/rewriter/poly_rewriter_def.h:570-586.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

// The shared cold binding borrows its manager and expression keys. Native map
// storage acquires no expression references or simulated capacity reservations.
type bvPolynomialPositions struct {
	manager *ast.ASTManager
	entries map[ast.ASTHandle]uint32
	header  uint64
	closed  bool
}

func newBVPolynomialPositions(m *ast.ASTManager) (*bvPolynomialPositions, error) {
	header := uint64(reflect.TypeOf(bvPolynomialPositions{}).Size())
	if e := m.Budget().Reserve(header); e != nil {
		return nil, e
	}
	return &bvPolynomialPositions{manager: m, entries: make(map[ast.ASTHandle]uint32), header: header}, nil
}

func (p *bvPolynomialPositions) ready() error {
	if p == nil || p.closed {
		return ast.ErrASTPending
	}
	return nil
}

func (p *bvPolynomialPositions) find(key ast.ASTHandle) (uint32, bool, error) {
	if e := p.ready(); e != nil {
		return 0, false, e
	}
	if _, e := p.manager.GetHash(key); e != nil {
		return 0, false, e
	}
	position, found := p.entries[key]
	return position, found, nil
}

func (p *bvPolynomialPositions) insert(key ast.ASTHandle, position uint32) error {
	if e := p.ready(); e != nil {
		return e
	}
	if _, e := p.manager.GetHash(key); e != nil {
		return e
	}
	p.entries[key] = position
	return nil
}

func (p *bvPolynomialPositions) reset() error {
	if e := p.ready(); e != nil {
		return e
	}
	clear(p.entries)
	return nil
}

func (p *bvPolynomialPositions) close() {
	if p == nil || p.closed {
		return
	}
	p.manager.Budget().Release(p.header)
	p.entries = nil
	p.header = 0
	p.closed = true
}
