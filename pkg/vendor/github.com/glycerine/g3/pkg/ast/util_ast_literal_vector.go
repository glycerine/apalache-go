package ast

// Source: src/util/sorting_network.h:120-1046; src/test/sorting_network.cpp:150-181.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// ASTLiteralVector stores borrowed, pointer-free AST handles. Its native slice
// does not own AST references or allocate through the numerical Runtime.
type ASTLiteralVector struct {
	self                            *ASTLiteralVector
	values                          []ASTHandle
	budget                          *arena.Budget
	header                          uint64
	constructed, sourceDone, closed bool
}

func NewASTLiteralVector(b *arena.Budget) (*ASTLiteralVector, error) {
	if b == nil {
		return nil, arena.ErrConfig
	}
	header := uint64(reflect.TypeOf(ASTLiteralVector{}).Size())
	if e := b.Reserve(header); e != nil {
		return nil, e
	}
	v := &ASTLiteralVector{budget: b, header: header, constructed: true}
	v.self = v
	return v, nil
}
func (v *ASTLiteralVector) open() error {
	if v == nil || v.self != v {
		return ErrASTHandle
	}
	if v.closed || !v.constructed || v.sourceDone {
		return arena.ErrClosed
	}
	return nil
}
func (v *ASTLiteralVector) Size() (uint32, error) {
	if e := v.open(); e != nil {
		return 0, e
	}
	return uint32(len(v.values)), nil
}
func (v *ASTLiteralVector) index(i uint32) error {
	if e := v.open(); e != nil {
		return e
	}
	if uint64(i) >= uint64(len(v.values)) {
		return arena.ErrBounds
	}
	return nil
}
func (v *ASTLiteralVector) At(i uint32) (ASTHandle, error) {
	if e := v.index(i); e != nil {
		return ASTHandle{}, e
	}
	return v.values[i], nil
}
func (v *ASTLiteralVector) Set(i uint32, h ASTHandle) error {
	if e := v.index(i); e != nil {
		return e
	}
	v.values[i] = h
	return nil
}
func (v *ASTLiteralVector) Push(h ASTHandle) error {
	if e := v.open(); e != nil {
		return e
	}
	v.values = append(v.values, h)
	return nil
}
func (v *ASTLiteralVector) Reset() error {
	if e := v.open(); e != nil {
		return e
	}
	clear(v.values)
	v.values = v.values[:0]
	return nil
}

// Span captures the current slice. Later growth keeps this borrow readable;
// source finalization and close invalidate every capture from this owner.
func (v *ASTLiteralVector) Span() (ASTLiteralVectorSpan, error) {
	if e := v.open(); e != nil {
		return ASTLiteralVectorSpan{}, e
	}
	return ASTLiteralVectorSpan{owner: v, values: v.values}, nil
}

type ASTLiteralVectorSpan struct {
	owner  *ASTLiteralVector
	values []ASTHandle
}

func (s ASTLiteralVectorSpan) At(i uint32) (ASTHandle, error) {
	if s.owner != nil {
		if e := s.owner.open(); e != nil {
			return ASTHandle{}, e
		}
	}
	if uint64(i) >= uint64(len(s.values)) {
		return ASTHandle{}, arena.ErrBounds
	}
	return s.values[i], nil
}
func (v *ASTLiteralVector) SourceFinalize() error {
	if v == nil || v.self != v {
		return ErrASTHandle
	}
	if v.closed || v.sourceDone {
		return nil
	}
	v.values = nil
	v.sourceDone = true
	return nil
}
func (v *ASTLiteralVector) Close() error {
	if v == nil || v.self != v {
		return ErrASTHandle
	}
	if v.closed {
		return nil
	}
	if e := v.SourceFinalize(); e != nil {
		return e
	}
	v.budget.Release(v.header)
	v.header = 0
	v.closed = true
	return nil
}
func (v *ASTLiteralVector) PopBack() error {
	if e := v.open(); e != nil {
		return e
	}
	if len(v.values) == 0 {
		return arena.ErrBounds
	}
	last := len(v.values) - 1
	v.values[last] = ASTHandle{}
	v.values = v.values[:last]
	return nil
}
func (v *ASTLiteralVector) Append(src *ASTLiteralVector) error {
	if e := v.open(); e != nil {
		return e
	}
	if e := src.open(); e != nil {
		return e
	}
	v.values = append(v.values, src.values...)
	return nil
}
