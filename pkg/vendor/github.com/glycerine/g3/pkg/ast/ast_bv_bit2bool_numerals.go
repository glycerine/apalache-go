package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:2212-2236;
// src/util/rational.h:43,46,462-464.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorBit2BoolNumerals retains v and reached literal/subtraction return
// temporaries through the caller's complete assignment expression.
type BitVectorBit2BoolNumerals struct {
	self                   *BitVectorBit2BoolNumerals
	manager                *ASTManager
	values                 [3]unumeral.OwnedRational // v, literal one, subtraction return
	placed                 [3]bool
	scalar                 unumeral.RationalServiceInvocation
	call                   ASTBitVectorNumeralInvocation
	header                 uint64
	sourceCause            error
	stopped, ended, closed bool
}

func NewBitVectorBit2BoolNumerals(m *ASTManager) (*BitVectorBit2BoolNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorBit2BoolNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorBit2BoolNumerals{manager: m, header: bytes}
	n.self = n
	v, e := m.numerals.NewValue()
	if e != nil {
		n.stopped = true
		return n, e
	}
	n.values[0] = v
	n.placed[0] = true
	return n, nil
}
func (n *BitVectorBit2BoolNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorBit2BoolNumerals) retire() error {
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
func (n *BitVectorBit2BoolNumerals) Read(arg ASTHandle) (yes bool, result error) {
	if e := n.open(); e != nil {
		return false, e
	}
	defer func() {
		if result != nil {
			n.stopped = true
		}
	}()
	var width uint32
	yes, result = n.manager.BitVectorNumeralValueWithInvocation(arg, &n.values[0], &width, &n.scalar)
	if result == nil {
		result = n.retire()
	}
	return
}
func (n *BitVectorBit2BoolNumerals) IsOne() (bool, error) {
	if e := n.open(); e != nil {
		return false, e
	}
	return n.manager.numerals.IsOne(&n.values[0])
}
func (n *BitVectorBit2BoolNumerals) IsZero() (bool, error) {
	if e := n.open(); e != nil {
		return false, e
	}
	return n.manager.numerals.IsZero(&n.values[0])
}
func (n *BitVectorBit2BoolNumerals) LiteralOne(subtract bool) (node ASTHandle, result error) {
	if e := n.open(); e != nil {
		return node, e
	}
	if n.placed[1] || n.placed[2] {
		return node, ErrASTPending
	}
	defer func() {
		if result != nil {
			n.stopped = true
		}
	}()
	q := n.manager.numerals
	v, e := q.NewValue()
	if e != nil {
		return node, e
	}
	n.values[1] = v
	n.placed[1] = true
	if e = q.SetInt64WithInvocation(&n.values[1], 1, &n.scalar); e != nil {
		return node, e
	}
	if e = n.retire(); e != nil {
		return node, e
	}
	index := 1
	if subtract {
		n.values[2], e = q.SubWithInvocation(&n.values[1], &n.values[0], &n.scalar)
		n.placed[2] = n.values[2] != (unumeral.OwnedRational{})
		if e != nil {
			return node, e
		}
		if e = n.retire(); e != nil {
			return node, e
		}
		index = 2
	}
	m := n.manager
	if e = m.lock(); e != nil {
		return node, e
	}
	defer m.mu.Unlock()
	if e = n.call.prepare(m); e != nil {
		return node, e
	}
	defer n.call.finish(&result)
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	r, e := m.mkBitVectorNumeralCaptured(&n.values[index], 1, &n.call.receipt)
	return m.external(r), e
}
func (n *BitVectorBit2BoolNumerals) EndExpression() error {
	if e := n.open(); e != nil {
		return e
	}
	e := n.manager.BitVectorNumeralInvocationSourceReady(&n.call)
	if e == nil {
		e = n.scalar.SourceReady()
	}
	for i := 2; i >= 1 && e == nil; i-- {
		if n.placed[i] {
			e = n.manager.numerals.Del(&n.values[i])
			if e == nil {
				n.placed[i] = false
			}
		}
	}
	if e != nil {
		n.stopped = true
		n.sourceCause = e
	}
	return e
}
func (n *BitVectorBit2BoolNumerals) SourceEnd() error {
	if e := n.EndExpression(); e != nil {
		return e
	}
	if n.placed[0] {
		if e := n.manager.numerals.Del(&n.values[0]); e != nil {
			n.stopped = true
			n.sourceCause = e
			return e
		}
		n.placed[0] = false
	}
	n.ended = true
	return nil
}
func (n *BitVectorBit2BoolNumerals) Close() error {
	if n == nil || n.self != n {
		return ErrASTHandle
	}
	if n.closed {
		return nil
	}
	if e := n.manager.CleanupBitVectorNumeralInvocation(&n.call); e != nil {
		return e
	}
	if e := n.scalar.Cleanup(false); e != nil {
		return e
	}
	for i := 2; i >= 0; i-- {
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
