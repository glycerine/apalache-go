package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:590-666,668-690;
// src/util/util.h:133-139.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorExtractNumerals owns default v/val and each reached power return
// or ui64 literal. The same concrete shape serves the propagation helper's
// copied val without introducing a second default numeral into that scope.
type BitVectorExtractNumerals struct {
	self                   *BitVectorExtractNumerals
	manager                *ASTManager
	values                 [3]unumeral.OwnedRational
	placed                 [3]bool
	scalar                 unumeral.RationalServiceInvocation
	call                   ASTBitVectorNumeralInvocation
	header                 uint64
	sourceCause            error
	stopped, ended, closed bool
	expressionClosed       bool
}

func NewBitVectorExtractNumerals(m *ASTManager) (*BitVectorExtractNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorExtractNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorExtractNumerals{manager: m, header: bytes}
	n.self = n
	v, e := m.numerals.NewValue()
	if e != nil {
		return n, n.fail(e)
	}
	n.values[0], n.placed[0] = v, true
	return n, nil
}
func (n *BitVectorExtractNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended || n.expressionClosed {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorExtractNumerals) fail(e error) error {
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorExtractNumerals) retire() error {
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
func (n *BitVectorExtractNumerals) Read(arg ASTHandle, width *uint32) (bool, error) {
	if e := n.open(); e != nil {
		return false, e
	}
	yes, e := n.manager.BitVectorNumeralValueWithInvocation(arg, &n.values[0], width, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return yes, n.fail(e)
}

func (n *BitVectorExtractNumerals) power(width uint32, modulo bool) error {
	if n.placed[1] {
		return ErrASTPending
	}
	q := n.manager.numerals
	v, e := q.PowerOfTwoWithInvocation(width, &n.scalar)
	n.values[1], n.placed[1] = v, v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	if modulo {
		e = q.ModIntoWithInvocation(&n.values[0], &n.values[1], &n.values[0], &n.scalar)
	} else {
		e = q.IDivIntoWithInvocation(&n.values[0], &n.values[1], &n.values[0], &n.scalar)
	}
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	return n.End(1)
}

// Evaluate preserves negative modulo at result width before the uint64 test,
// saturating machine shifts, and the big-number void division full expression.
// A reached ui64 literal remains alive through the caller's output assignment.
func (n *BitVectorExtractNumerals) Evaluate(high, low uint32) (ASTHandle, error) {
	if e := n.open(); e != nil {
		return ASTHandle{}, e
	}
	if n.placed[1] || n.placed[2] {
		return ASTHandle{}, ErrASTPending
	}
	q := n.manager.numerals
	bits := high - low + 1
	negative, e := q.IsNeg(&n.values[0])
	if e != nil {
		return ASTHandle{}, n.fail(e)
	}
	if negative {
		if e = n.power(bits, true); e != nil {
			return ASTHandle{}, e
		}
	}
	small, e := q.IsUint64(&n.values[0])
	if e != nil {
		return ASTHandle{}, n.fail(e)
	}
	if small {
		value, e := q.Uint64(&n.values[0])
		if e != nil {
			return ASTHandle{}, n.fail(e)
		}
		// Go's uint64 shifts match original shift_right/shift_left even >=64.
		value = (value >> low) & ((uint64(1) << bits) - 1)
		v, e := q.NewValue()
		if e != nil {
			return ASTHandle{}, n.fail(e)
		}
		n.values[2], n.placed[2] = v, true
		e = q.SetUint64WithInvocation(&n.values[2], value, &n.scalar)
		if e == nil {
			e = n.retire()
		}
		if e != nil {
			return ASTHandle{}, n.fail(e)
		}
		return n.Constant(2, bits)
	}
	if e = n.power(low, false); e != nil {
		return ASTHandle{}, e
	}
	return n.Constant(0, bits)
}

func (n *BitVectorExtractNumerals) Constant(i int, width uint32) (node ASTHandle, result error) {
	if e := n.open(); e != nil {
		return node, e
	}
	if i < 0 || i >= len(n.values) || !n.placed[i] {
		return node, ErrASTPending
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
func (n *BitVectorExtractNumerals) End(indices ...int) error {
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
func (n *BitVectorExtractNumerals) SourceEnd() error {
	if e := n.End(2, 1, 0); e != nil {
		return e
	}
	n.ended = true
	return nil
}
func (n *BitVectorExtractNumerals) CloseExpression() error {
	if n == nil || n.self != n {
		return ErrASTHandle
	}
	if n.closed || n.expressionClosed {
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
	n.expressionClosed = true
	return nil
}
func (n *BitVectorExtractNumerals) Close() error {
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
