package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:1146-1153.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorMinusOneNumerals owns the original default r and reached comparison
// full expression. It compares the stored value without normalization.
type BitVectorMinusOneNumerals struct {
	self                   *BitVectorMinusOneNumerals
	manager                *ASTManager
	values                 [4]unumeral.OwnedRational
	placed                 [4]bool
	scalar                 unumeral.RationalServiceInvocation
	header                 uint64
	sourceCause            error
	stopped, ended, closed bool
}

func NewBitVectorMinusOneNumerals(m *ASTManager) (*BitVectorMinusOneNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorMinusOneNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorMinusOneNumerals{manager: m, header: bytes}
	n.self = n
	v, e := m.numerals.NewValue()
	if e != nil {
		return n, n.fail(e)
	}
	n.values[0] = v
	n.placed[0] = true
	return n, nil
}
func (n *BitVectorMinusOneNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorMinusOneNumerals) fail(e error) error {
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorMinusOneNumerals) retire() error {
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
func (n *BitVectorMinusOneNumerals) Read(arg ASTHandle, width *uint32) (bool, error) {
	if e := n.open(); e != nil {
		return false, e
	}
	yes, e := n.manager.BitVectorNumeralValueWithInvocation(arg, &n.values[0], width, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return yes, n.fail(e)
}
func (n *BitVectorMinusOneNumerals) Compare(width uint32) (bool, error) {
	if e := n.open(); e != nil {
		return false, e
	}
	for _, placed := range n.placed[1:] {
		if placed {
			return false, ErrASTPending
		}
	}
	q := n.manager.numerals
	v, e := q.PowerOfTwoWithInvocation(width, &n.scalar)
	n.values[1] = v
	n.placed[1] = v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return false, n.fail(e)
	}
	v, e = q.NewValue()
	if e != nil {
		return false, n.fail(e)
	}
	n.values[2] = v
	n.placed[2] = true
	e = q.SetInt64WithInvocation(&n.values[2], 1, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return false, n.fail(e)
	}
	v, e = q.SubWithInvocation(&n.values[1], &n.values[2], &n.scalar)
	n.values[3] = v
	n.placed[3] = v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return false, n.fail(e)
	}
	yes, e := q.Equal(&n.values[0], &n.values[3])
	return yes, n.fail(e)
}

// SourceEnd follows return-expression evaluation: subtraction, literal one,
// power, and finally the older function-local r.
func (n *BitVectorMinusOneNumerals) SourceEnd() error {
	if e := n.open(); e != nil {
		return e
	}
	if e := n.scalar.SourceReady(); e != nil {
		n.sourceCause = e
		return n.fail(e)
	}
	for i := len(n.values) - 1; i >= 0; i-- {
		if n.placed[i] {
			if e := n.manager.numerals.Del(&n.values[i]); e != nil {
				n.sourceCause = e
				return n.fail(e)
			}
			n.placed[i] = false
		}
	}
	n.ended = true
	return nil
}
func (n *BitVectorMinusOneNumerals) Close() error {
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
