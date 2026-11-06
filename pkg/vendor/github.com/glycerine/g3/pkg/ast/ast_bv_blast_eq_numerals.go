package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:2287-2302;
// src/util/rational.h:43,46,182-184,202-206.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorBlastEqualityNumerals owns physical v, reached two and each reached
// remainder return. Division mutates v directly after the caller's buffer push.
type BitVectorBlastEqualityNumerals struct {
	self                   *BitVectorBlastEqualityNumerals
	manager                *ASTManager
	values                 [3]unumeral.OwnedRational // v, two, remainder return
	placed                 [3]bool
	scalar                 unumeral.RationalServiceInvocation
	call                   ASTBitVectorNumeralInvocation
	header                 uint64
	sourceCause            error
	stopped, ended, closed bool
}

func NewBitVectorBlastEqualityNumerals(m *ASTManager) (*BitVectorBlastEqualityNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorBlastEqualityNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorBlastEqualityNumerals{manager: m, header: bytes}
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
func (n *BitVectorBlastEqualityNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorBlastEqualityNumerals) retire() error {
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
func (n *BitVectorBlastEqualityNumerals) Read(arg ASTHandle, width *uint32) (yes bool, result error) {
	if e := n.open(); e != nil {
		return false, e
	}
	defer func() {
		if result != nil {
			n.stopped = true
		}
	}()
	yes, result = n.manager.BitVectorNumeralValueWithInvocation(arg, &n.values[0], width, &n.scalar)
	if result == nil {
		result = n.retire()
	}
	return
}
func (n *BitVectorBlastEqualityNumerals) BeginTwo() (result error) {
	if e := n.open(); e != nil {
		return e
	}
	if n.placed[1] {
		return ErrASTPending
	}
	defer func() {
		if result != nil {
			n.stopped = true
		}
	}()
	v, e := n.manager.numerals.NewValue()
	if e != nil {
		return e
	}
	n.values[1] = v
	n.placed[1] = true
	result = n.manager.numerals.SetInt64WithInvocation(&n.values[1], 2, &n.scalar)
	if result == nil {
		result = n.retire()
	}
	return
}
func (n *BitVectorBlastEqualityNumerals) RemainderIsZero() (zero bool, result error) {
	if e := n.open(); e != nil {
		return false, e
	}
	if !n.placed[1] || n.placed[2] {
		return false, ErrASTPending
	}
	defer func() {
		if result != nil {
			n.stopped = true
		}
	}()
	q := n.manager.numerals
	n.values[2], result = q.RemWithInvocation(&n.values[0], &n.values[1], &n.scalar)
	n.placed[2] = n.values[2] != (unumeral.OwnedRational{})
	if result != nil {
		return
	}
	if result = n.retire(); result != nil {
		return
	}
	zero, result = q.IsZero(&n.values[2])
	if result != nil {
		return
	}
	result = q.Del(&n.values[2])
	if result == nil {
		n.placed[2] = false
	} else {
		n.sourceCause = result
	}
	return
}
func (n *BitVectorBlastEqualityNumerals) Divide() (result error) {
	if e := n.open(); e != nil {
		return e
	}
	if !n.placed[1] || n.placed[2] {
		return ErrASTPending
	}
	defer func() {
		if result != nil {
			n.stopped = true
		}
	}()
	result = n.manager.numerals.IDivIntoWithInvocation(&n.values[0], &n.values[1], &n.values[0], &n.scalar)
	if result == nil {
		result = n.retire()
	}
	return
}
func (n *BitVectorBlastEqualityNumerals) Bit(zero bool) (node ASTHandle, result error) {
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
	if result = n.call.prepare(m); result != nil {
		return
	}
	defer n.call.finish(&result)
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	value := uint32(1)
	if zero {
		value = 0
	}
	r, e := m.bitVectorNumeralUnsignedCaptured(value, 1, &n.call.receipt)
	return m.external(r), e
}
func (n *BitVectorBlastEqualityNumerals) SourceEnd() error {
	if e := n.open(); e != nil {
		return e
	}
	e := n.scalar.SourceReady()
	if e == nil {
		e = n.manager.BitVectorNumeralInvocationSourceReady(&n.call)
	}
	for i := 2; i >= 0 && e == nil; i-- {
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
		return e
	}
	n.ended = true
	return nil
}
func (n *BitVectorBlastEqualityNumerals) Close() error {
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
