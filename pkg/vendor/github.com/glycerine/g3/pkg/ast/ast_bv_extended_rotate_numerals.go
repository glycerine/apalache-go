package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:2020-2038.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorExtendedRotateNumerals owns default r2 and the reached unsigned
// width literal and remainder return. Stored values are read without normalization.
type BitVectorExtendedRotateNumerals struct {
	self                   *BitVectorExtendedRotateNumerals
	manager                *ASTManager
	values                 [3]unumeral.OwnedRational
	placed                 [3]bool
	scalar                 unumeral.RationalServiceInvocation
	header                 uint64
	sourceCause            error
	stopped, ended, closed bool
}

func NewBitVectorExtendedRotateNumerals(m *ASTManager) (*BitVectorExtendedRotateNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorExtendedRotateNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorExtendedRotateNumerals{manager: m, header: bytes}
	n.self = n
	v, e := m.numerals.NewValue()
	if e != nil {
		return n, n.fail(e)
	}
	n.values[0] = v
	n.placed[0] = true
	return n, nil
}
func (n *BitVectorExtendedRotateNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorExtendedRotateNumerals) fail(e error) error {
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorExtendedRotateNumerals) retire() error {
	if e := n.scalar.SourceReady(); e != nil {
		return e
	}
	if e := n.scalar.Cleanup(false); e != nil {
		return e
	}
	if e := n.scalar.Cleanup(true); e != nil {
		return e
	}
	if e := n.scalar.EndContext(); e != nil {
		return e
	}
	n.scalar = unumeral.RationalServiceInvocation{}
	return nil
}
func (n *BitVectorExtendedRotateNumerals) Read(arg ASTHandle, width *uint32) (bool, error) {
	if e := n.open(); e != nil {
		return false, e
	}
	yes, e := n.manager.BitVectorNumeralValueWithInvocation(arg, &n.values[0], width, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return yes, n.fail(e)
}

// Shift evaluates the original remainder/get_uint64/modulo full expression.
// The returned remainder ends before the width literal, and older r2 remains
// alive while the caller delegates to fixed rotation and publishes its result.
func (n *BitVectorExtendedRotateNumerals) Shift(width uint32) (uint32, error) {
	if e := n.open(); e != nil {
		return 0, e
	}
	if n.placed[1] || n.placed[2] {
		return 0, ErrASTPending
	}
	q := n.manager.numerals
	v, e := q.NewValue()
	if e != nil {
		return 0, n.fail(e)
	}
	n.values[1] = v
	n.placed[1] = true
	e = q.SetUnsignedWithInvocation(&n.values[1], width, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return 0, n.fail(e)
	}
	v, e = q.RemWithInvocation(&n.values[0], &n.values[1], &n.scalar)
	n.values[2] = v
	n.placed[2] = v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return 0, n.fail(e)
	}
	value, e := q.Uint64(&n.values[2])
	if e != nil {
		return 0, n.fail(e)
	}
	if width == 0 {
		return 0, n.fail(ErrASTDomain)
	}
	shift := uint32(value % uint64(width))
	if e = n.End(2, 1); e != nil {
		return 0, e
	}
	return shift, nil
}
func (n *BitVectorExtendedRotateNumerals) End(indices ...int) error {
	if e := n.open(); e != nil {
		return e
	}
	if e := n.scalar.SourceReady(); e != nil {
		n.sourceCause = e
		return n.fail(e)
	}
	for _, i := range indices {
		if i < 0 || i >= len(n.values) {
			return ErrASTHandle
		}
		if n.placed[i] {
			if e := n.manager.numerals.Del(&n.values[i]); e != nil {
				n.sourceCause = e
				return n.fail(e)
			}
			n.placed[i] = false
		}
	}
	return nil
}
func (n *BitVectorExtendedRotateNumerals) SourceEnd() error {
	if e := n.End(2, 1, 0); e != nil {
		return e
	}
	n.ended = true
	return nil
}
func (n *BitVectorExtendedRotateNumerals) Close() error {
	if n == nil || n.self != n {
		return ErrASTHandle
	}
	if n.closed {
		return nil
	}
	if e := n.scalar.Cleanup(false); e != nil {
		return e
	}
	for i := len(n.values) - 1; i >= 0; i-- {
		if n.placed[i] {
			if e := n.manager.numerals.CleanupValue(&n.values[i]); e != nil {
				return e
			}
			n.placed[i] = false
		}
	}
	if e := n.scalar.Cleanup(true); e != nil {
		return e
	}
	if e := n.scalar.EndContext(); e != nil {
		return e
	}
	n.manager.Budget().Release(n.header)
	n.header = 0
	n.closed = true
	return nil
}
