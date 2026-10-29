package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:2190-2203;
// src/ast/rewriter/bv_rewriter.h:177; src/util/rational.h:43,48,298-300.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorMultiplicationNumerals owns the reached default v and exact
// copied-numeral child. The zero literal factory ends before concat starts.
type BitVectorMultiplicationNumerals struct {
	self                           *BitVectorMultiplicationNumerals
	manager                        *ASTManager
	value                          unumeral.OwnedRational
	scalar                         unumeral.RationalServiceInvocation
	zero                           ASTBitVectorNumeralInvocation
	header                         uint64
	sourceCause                    error
	placed, stopped, ended, closed bool
}

func NewBitVectorMultiplicationNumerals(m *ASTManager) (*BitVectorMultiplicationNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorMultiplicationNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorMultiplicationNumerals{manager: m, header: bytes}
	n.self = n
	v, e := m.numerals.NewValue()
	if e != nil {
		n.stopped = true
		return n, e
	}
	n.value = v
	n.placed = true
	return n, nil
}
func (n *BitVectorMultiplicationNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorMultiplicationNumerals) ReadPower(arg ASTHandle) (yes bool, width, shift uint32, result error) {
	if e := n.open(); e != nil {
		return false, 0, 0, e
	}
	defer func() {
		if result != nil {
			n.stopped = true
		}
	}()
	yes, result = n.manager.BitVectorNumeralValueWithInvocation(arg, &n.value, &width, &n.scalar)
	if result != nil {
		return
	}
	if result = n.scalar.SourceReady(); result != nil {
		return
	}
	if result = n.scalar.Cleanup(false); result != nil {
		return
	}
	if result = n.scalar.Cleanup(true); result != nil {
		return
	}
	if result = n.scalar.EndContext(); result != nil {
		return
	}
	n.scalar = unumeral.RationalServiceInvocation{}
	if yes {
		yes, shift, result = n.manager.numerals.IsPowerOfTwo(&n.value)
	}
	return
}
func (n *BitVectorMultiplicationNumerals) Zero(width uint32) (node ASTHandle, result error) {
	if e := n.open(); e != nil {
		return node, e
	}
	defer func() {
		if result != nil {
			n.stopped = true
		}
	}()
	m := n.manager
	if result = m.lock(); result != nil {
		return
	}
	defer m.mu.Unlock()
	if result = n.zero.prepare(m); result != nil {
		return
	}
	defer n.zero.finish(&result)
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	r, e := m.bitVectorNumeralUnsignedCaptured(0, width, &n.zero.receipt)
	return m.external(r), e
}
func (n *BitVectorMultiplicationNumerals) SourceEnd() error {
	if e := n.open(); e != nil {
		return e
	}
	var e error
	if e = n.scalar.SourceReady(); e == nil {
		e = n.manager.BitVectorNumeralInvocationSourceReady(&n.zero)
	}
	if e == nil && n.placed {
		e = n.manager.numerals.Del(&n.value)
		if e == nil {
			n.placed = false
		}
	}
	if e != nil {
		n.stopped = true
		n.sourceCause = e
		return e
	}
	n.ended = true
	return nil
}
func (n *BitVectorMultiplicationNumerals) Close() error {
	if n == nil || n.self != n {
		return ErrASTHandle
	}
	if n.closed {
		return nil
	}
	if e := n.manager.CleanupBitVectorNumeralInvocation(&n.zero); e != nil {
		return e
	}
	if e := n.scalar.Cleanup(false); e != nil {
		return e
	}
	if n.placed {
		if e := n.manager.numerals.CleanupValue(&n.value); e != nil {
			return e
		}
		n.placed = false
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
