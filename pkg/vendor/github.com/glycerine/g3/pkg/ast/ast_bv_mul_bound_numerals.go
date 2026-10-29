package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:2706-2800.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorMultiplyBoundNumerals retains original operand/limit/product locals.
type BitVectorMultiplyBoundNumerals struct {
	self                   *BitVectorMultiplyBoundNumerals
	manager                *ASTManager
	values                 [6]unumeral.OwnedRational
	placed                 [6]bool
	scalar                 unumeral.RationalServiceInvocation
	header                 uint64
	sourceCause            error
	stopped, ended, closed bool
}

func NewBitVectorMultiplyBoundNumerals(m *ASTManager) (*BitVectorMultiplyBoundNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorMultiplyBoundNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorMultiplyBoundNumerals{manager: m, header: bytes}
	n.self = n
	for _, i := range []int{0, 1} {
		if e := n.Place(i); e != nil {
			return n, e
		}
	}
	return n, nil
}
func (n *BitVectorMultiplyBoundNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorMultiplyBoundNumerals) slot(i int) error {
	if e := n.open(); e != nil {
		return e
	}
	if i < 0 || i >= len(n.values) || !n.placed[i] {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorMultiplyBoundNumerals) fail(e error) error {
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorMultiplyBoundNumerals) Place(i int) error {
	if e := n.open(); e != nil {
		return e
	}
	if i < 0 || i >= len(n.values) || n.placed[i] {
		return ErrASTPending
	}
	v, e := n.manager.numerals.NewValue()
	if e != nil {
		return n.fail(e)
	}
	n.values[i] = v
	n.placed[i] = true
	return nil
}
func (n *BitVectorMultiplyBoundNumerals) retire() error {
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
func (n *BitVectorMultiplyBoundNumerals) Read(i int, arg ASTHandle, width *uint32) (bool, error) {
	if e := n.slot(i); e != nil {
		return false, e
	}
	yes, e := n.manager.BitVectorNumeralValueWithInvocation(arg, &n.values[i], width, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return yes, n.fail(e)
}

func (n *BitVectorMultiplyBoundNumerals) ZeroOrOne(i int) (bool, error) {
	if e := n.slot(i); e != nil {
		return false, e
	}
	yes, e := n.manager.numerals.IsZero(&n.values[i])
	if e != nil || yes {
		return yes, n.fail(e)
	}
	yes, e = n.manager.numerals.IsOne(&n.values[i])
	return yes, n.fail(e)
}
func (n *BitVectorMultiplyBoundNumerals) Power(i int, width uint32) error {
	if e := n.open(); e != nil {
		return e
	}
	if i < 2 || i >= 6 || n.placed[i] {
		return ErrASTPending
	}
	v, e := n.manager.numerals.PowerOfTwoWithInvocation(width, &n.scalar)
	n.values[i] = v
	n.placed[i] = v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	return n.fail(e)
}
func (n *BitVectorMultiplyBoundNumerals) Product(i int) error {
	if e := n.open(); e != nil {
		return e
	}
	if i < 2 || i >= 6 || n.placed[i] {
		return ErrASTPending
	}
	v, e := n.manager.numerals.MulWithInvocation(&n.values[0], &n.values[1], &n.scalar)
	n.values[i] = v
	n.placed[i] = v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	return n.fail(e)
}
func (n *BitVectorMultiplyBoundNumerals) AdjustSigned(i int) error {
	if e := n.slot(i); e != nil {
		return e
	}
	yes, e := n.manager.numerals.GreaterEqual(&n.values[i], &n.values[3])
	if e != nil {
		return n.fail(e)
	}
	if !yes {
		return nil
	}
	e = n.manager.numerals.SubAssignWithInvocation(&n.values[i], &n.values[2], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return n.fail(e)
}
func (n *BitVectorMultiplyBoundNumerals) NegativeLimit() error {
	if e := n.slot(3); e != nil {
		return e
	}
	if n.placed[5] {
		return ErrASTPending
	}
	v, e := n.manager.numerals.NegWithInvocation(&n.values[3], &n.scalar)
	n.values[5] = v
	n.placed[5] = v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	return n.fail(e)
}

// SignedUnderflowTraceValues reads the four live locals at the source TRACE site.
func (n *BitVectorMultiplyBoundNumerals) SignedUnderflowTraceValues() ([4]string, error) {
	var text [4]string
	for i, slot := range [...]int{0, 1, 4, 5} {
		if e := n.slot(slot); e != nil {
			return text, e
		}
		value, e := n.manager.numerals.String(&n.values[slot])
		if e != nil {
			return text, n.fail(e)
		}
		text[i] = value
	}
	return text, nil
}
func (n *BitVectorMultiplyBoundNumerals) Compare(a, b int, greaterEqual bool) (bool, error) {
	if e := n.slot(a); e != nil {
		return false, e
	}
	if e := n.slot(b); e != nil {
		return false, e
	}
	var yes bool
	var e error
	if greaterEqual {
		yes, e = n.manager.numerals.GreaterEqual(&n.values[a], &n.values[b])
	} else {
		yes, e = n.manager.numerals.Less(&n.values[a], &n.values[b])
	}
	return yes, n.fail(e)
}

// End receives explicit reverse declaration/full-expression order from the caller.
func (n *BitVectorMultiplyBoundNumerals) End(indices ...int) error {
	if e := n.open(); e != nil {
		return e
	}
	e := n.scalar.SourceReady()
	if e != nil {
		n.sourceCause = e
		return n.fail(e)
	}
	for _, i := range indices {
		if i < 0 || i >= len(n.values) {
			return ErrASTHandle
		}
		if n.placed[i] {
			if e = n.manager.numerals.Del(&n.values[i]); e != nil {
				n.sourceCause = e
				return n.fail(e)
			}
			n.placed[i] = false
		}
	}
	return nil
}
func (n *BitVectorMultiplyBoundNumerals) SourceEnd() error {
	if e := n.End(5, 4, 3, 2, 1, 0); e != nil {
		return e
	}
	n.ended = true
	return nil
}
func (n *BitVectorMultiplyBoundNumerals) Close() error {
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
