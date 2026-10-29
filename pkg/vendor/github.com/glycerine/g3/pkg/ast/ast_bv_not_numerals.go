package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:1905-1969; src/ast/bv_decl_plugin.cpp:753-765.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorNotNumerals retains default val, complement return, and reached
// power/literal/minus_one values in the original bvnot body.
type BitVectorNotNumerals struct {
	self                                     *BitVectorNotNumerals
	manager                                  *ASTManager
	values                                   [5]unumeral.OwnedRational
	placed                                   [5]bool
	scalar                                   unumeral.RationalServiceInvocation
	call                                     ASTBitVectorNumeralInvocation
	header                                   uint64
	sourceCause                              error
	stopped, ended, closed, expressionClosed bool
}

func NewBitVectorNotNumerals(m *ASTManager) (*BitVectorNotNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorNotNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorNotNumerals{manager: m, header: bytes}
	n.self = n
	for _, i := range []int{0} {
		if e := n.Place(i); e != nil {
			return n, e
		}
	}
	return n, nil
}

func (n *BitVectorNotNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended || n.expressionClosed {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorNotNumerals) slot(i int) error {
	if e := n.open(); e != nil {
		return e
	}
	if i < 0 || i >= len(n.values) || !n.placed[i] {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorNotNumerals) fail(e error) error {
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorNotNumerals) Place(i int) error {
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
func (n *BitVectorNotNumerals) retire() error {
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
func (n *BitVectorNotNumerals) Read(arg ASTHandle, width *uint32) (bool, error) {
	if e := n.slot(0); e != nil {
		return false, e
	}
	yes, e := n.manager.BitVectorNumeralValueWithInvocation(arg, &n.values[0], width, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return yes, n.fail(e)
}
func (n *BitVectorNotNumerals) Complement(width uint32) error {
	if e := n.slot(0); e != nil {
		return e
	}
	if n.placed[1] {
		return ErrASTPending
	}
	q := n.manager.numerals
	v, e := q.BitwiseNotWithInvocation(width, &n.values[0], &n.scalar)
	n.values[1] = v
	n.placed[1] = v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	e = q.CopyWithInvocation(&n.values[0], &n.values[1], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	return n.End(1)
}
func (n *BitVectorNotNumerals) Constant(i int, width uint32) (node ASTHandle, result error) {
	if e := n.slot(i); e != nil {
		return node, e
	}
	defer func() { n.fail(result) }()
	m := n.manager
	if e := m.lock(); e != nil {
		return node, e
	}
	defer m.mu.Unlock()
	if e := n.call.prepare(m); e != nil {
		return node, e
	}
	defer n.call.finish(&result)
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	r, e := m.mkBitVectorNumeralCaptured(&n.values[i], width, &n.call.receipt)
	return m.external(r), e
}

// End receives explicit reverse declaration/full-expression order from the caller.
func (n *BitVectorNotNumerals) End(indices ...int) error {
	if e := n.open(); e != nil {
		return e
	}
	e := n.scalar.SourceReady()
	if e == nil {
		e = n.manager.BitVectorNumeralInvocationSourceReady(&n.call)

	}
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
func (n *BitVectorNotNumerals) SourceEnd() error {
	if e := n.End(4, 3, 2, 1, 0); e != nil {
		return e
	}
	n.ended = true
	return nil
}

// CloseExpression joins numerical children and younger values before the
// structural caller ends its expression references and older default val.
func (n *BitVectorNotNumerals) CloseExpression() error {
	if n == nil || n.self != n {
		return ErrASTHandle
	}
	if n.expressionClosed || n.closed {
		return nil
	}
	if e := n.manager.CleanupBitVectorNumeralInvocation(&n.call); e != nil {
		return e
	}
	if e := n.scalar.Cleanup(false); e != nil {
		return e
	}
	for i := len(n.values) - 1; i >= 1; i-- {
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
func (n *BitVectorNotNumerals) Close() error {
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

// MinusOne ends its power/literal full-expression temporaries after the
// distinct subtraction return becomes the local minus_one. The simplification
// branch borrows static one; the arithmetic branch constructs numeral(1).
func (n *BitVectorNotNumerals) MinusOne(width uint32, borrowedOne bool) error {
	if e := n.open(); e != nil {
		return e
	}
	for _, i := range []int{2, 3, 4} {
		if n.placed[i] {
			return ErrASTPending
		}
	}
	q := n.manager.numerals
	v, e := q.PowerOfTwoWithInvocation(width, &n.scalar)
	n.values[2] = v
	n.placed[2] = v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	if borrowedOne {
		v, e = q.SubOneWithInvocation(&n.values[2], &n.scalar)
	} else {
		if e = n.Place(3); e != nil {
			return e
		}
		e = q.SetInt64WithInvocation(&n.values[3], 1, &n.scalar)
		if e == nil {
			e = n.retire()
		}
		if e != nil {
			return n.fail(e)
		}
		v, e = q.SubWithInvocation(&n.values[2], &n.values[3], &n.scalar)
	}
	n.values[4] = v
	n.placed[4] = v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	return n.End(3, 2)
}
