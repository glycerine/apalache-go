package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:1362-1406.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorBV2IntNumerals retains default v and reached literal two/power
// full expressions through arithmetic construction and caller publication.
type BitVectorBV2IntNumerals struct {
	self                   *BitVectorBV2IntNumerals
	manager                *ASTManager
	call                   ASTArithmeticNumeralInvocation
	expressionClosed       bool
	values                 [3]unumeral.OwnedRational
	placed                 [3]bool
	scalar                 unumeral.RationalServiceInvocation
	header                 uint64
	sourceCause            error
	stopped, ended, closed bool
}

func NewBitVectorBV2IntNumerals(m *ASTManager) (*BitVectorBV2IntNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorBV2IntNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorBV2IntNumerals{manager: m, header: bytes}
	n.self = n
	v, e := m.numerals.NewValue()
	if e != nil {
		return n, n.fail(e)
	}
	n.values[0] = v
	n.placed[0] = true
	return n, nil
}
func (n *BitVectorBV2IntNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended || n.expressionClosed {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorBV2IntNumerals) fail(e error) error {
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorBV2IntNumerals) retire() error {
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
func (n *BitVectorBV2IntNumerals) Read(arg ASTHandle, width *uint32) (bool, error) {
	if e := n.open(); e != nil {
		return false, e
	}
	yes, e := n.manager.BitVectorNumeralValueWithInvocation(arg, &n.values[0], width, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return yes, n.fail(e)
}
func (n *BitVectorBV2IntNumerals) End(indices ...int) error {
	if e := n.open(); e != nil {
		return e
	}
	if e := n.manager.ArithmeticNumeralInvocationSourceReady(&n.call); e != nil {
		n.sourceCause = e
		return n.fail(e)
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

// SourceEnd ends reached power expression values before older v.
func (n *BitVectorBV2IntNumerals) SourceEnd() error {
	if e := n.End(2, 1, 0); e != nil {
		return e
	}
	n.ended = true
	return nil
}
func (n *BitVectorBV2IntNumerals) CloseExpression() error {
	if n == nil || n.self != n {
		return ErrASTHandle
	}
	if n.expressionClosed || n.closed {
		return nil
	}
	if e := n.manager.CleanupArithmeticNumeralInvocation(&n.call); e != nil {
		return e
	}
	if e := n.scalar.Cleanup(false); e != nil {
		return e
	}
	for i := 2; i >= 1; i-- {
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
	n.expressionClosed = true
	return nil
}
func (n *BitVectorBV2IntNumerals) Close() error {
	if n == nil || n.self != n {
		return ErrASTHandle
	}
	if n.closed {
		return nil
	}
	if e := n.CloseExpression(); e != nil {
		return e
	}
	if n.placed[0] {
		if e := n.manager.numerals.CleanupValue(&n.values[0]); e != nil {
			return e
		}
		n.placed[0] = false
	}
	n.manager.Budget().Release(n.header)
	n.header = 0
	n.closed = true
	return nil
}
func (n *BitVectorBV2IntNumerals) Constant(i int) (ASTHandle, error) {
	if e := n.open(); e != nil {
		return ASTHandle{}, e
	}
	if i < 0 || i >= len(n.values) || !n.placed[i] {
		return ASTHandle{}, ErrASTPending
	}
	h, e := n.manager.ArithmeticNumeralWithInvocation(&n.values[i], true, &n.call)
	return h, n.fail(e)
}
func (n *BitVectorBV2IntNumerals) Zero() (ASTHandle, error) {
	if e := n.open(); e != nil {
		return ASTHandle{}, e
	}
	if n.placed[1] || n.placed[2] {
		return ASTHandle{}, ErrASTPending
	}
	v, e := n.manager.numerals.NewValue()
	if e != nil {
		return ASTHandle{}, n.fail(e)
	}
	n.values[1], n.placed[1] = v, true
	e = n.manager.numerals.SetInt64WithInvocation(&n.values[1], 0, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return ASTHandle{}, n.fail(e)
	}
	return n.Constant(1)
}
func (n *BitVectorBV2IntNumerals) Coefficient(width uint32) (ASTHandle, error) {
	if e := n.open(); e != nil {
		return ASTHandle{}, e
	}
	if n.placed[1] || n.placed[2] || n.expressionClosed {
		return ASTHandle{}, ErrASTPending
	}
	q := n.manager.numerals
	v, e := q.NewValue()
	if e != nil {
		return ASTHandle{}, n.fail(e)
	}
	n.values[1], n.placed[1] = v, true
	e = q.SetInt64WithInvocation(&n.values[1], 2, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return ASTHandle{}, n.fail(e)
	}
	v, e = q.PowerWithInvocation(&n.values[1], width, &n.scalar)
	n.values[2] = v
	n.placed[2] = v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return ASTHandle{}, n.fail(e)
	}
	return n.Constant(2)
}
