package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:1531-1540.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorSignExtensionNumerals owns default r, its normalization return
// and reached power temporary. The older r survives symbolic concat rewriting.
type BitVectorSignExtensionNumerals struct {
	self                   *BitVectorSignExtensionNumerals
	manager                *ASTManager
	values                 [3]unumeral.OwnedRational
	placed                 [3]bool
	scalar                 unumeral.RationalServiceInvocation
	call                   ASTBitVectorNumeralInvocation
	norm                   ASTBitVectorNumeralInvocation
	header                 uint64
	sourceCause            error
	stopped, ended, closed bool
	expressionClosed       bool
}

func NewBitVectorSignExtensionNumerals(m *ASTManager) (*BitVectorSignExtensionNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorSignExtensionNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorSignExtensionNumerals{manager: m, header: bytes}
	n.self = n
	v, e := m.numerals.NewValue()
	if e != nil {
		return n, n.fail(e)
	}
	n.values[0], n.placed[0] = v, true
	return n, nil
}
func (n *BitVectorSignExtensionNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended || n.expressionClosed {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorSignExtensionNumerals) fail(e error) error {
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorSignExtensionNumerals) retire() error {
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
func (n *BitVectorSignExtensionNumerals) Read(arg ASTHandle, width *uint32) (bool, error) {
	if e := n.open(); e != nil {
		return false, e
	}
	yes, e := n.manager.BitVectorNumeralValueWithInvocation(arg, &n.values[0], width, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return yes, n.fail(e)
}

func (n *BitVectorSignExtensionNumerals) Evaluate(amount, width uint32) (ASTHandle, error) {
	if e := n.open(); e != nil {
		return ASTHandle{}, e
	}
	if n.placed[1] || n.placed[2] {
		return ASTHandle{}, ErrASTPending
	}
	m := n.manager
	q := m.numerals
	resultWidth := width + amount
	v, e := m.NormalizeSignedBitVectorValueWithInvocation(&n.values[0], width, &n.norm)
	n.values[1], n.placed[1] = v, v != (unumeral.OwnedRational{})
	if e == nil {
		e = m.BitVectorNumeralInvocationSourceReady(&n.norm)
	}
	if e != nil {
		return ASTHandle{}, n.fail(e)
	}
	e = q.CopyWithInvocation(&n.values[0], &n.values[1], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return ASTHandle{}, n.fail(e)
	}
	if e = n.End(1); e != nil {
		return ASTHandle{}, e
	}
	v, e = q.PowerOfTwoWithInvocation(resultWidth, &n.scalar)
	n.values[2], n.placed[2] = v, v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return ASTHandle{}, n.fail(e)
	}
	e = q.ModIntoWithInvocation(&n.values[0], &n.values[2], &n.values[0], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return ASTHandle{}, n.fail(e)
	}
	if e = n.End(2); e != nil {
		return ASTHandle{}, e
	}
	return n.Constant(0, resultWidth)
}

func (n *BitVectorSignExtensionNumerals) Constant(i int, width uint32) (node ASTHandle, result error) {
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
func (n *BitVectorSignExtensionNumerals) End(indices ...int) error {
	if e := n.open(); e != nil {
		return e
	}
	e := n.scalar.SourceReady()
	if e == nil {
		e = n.manager.BitVectorNumeralInvocationSourceReady(&n.call)
		if e == nil {
			e = n.manager.BitVectorNumeralInvocationSourceReady(&n.norm)
		}
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
func (n *BitVectorSignExtensionNumerals) SourceEnd() error {
	if e := n.End(2, 1, 0); e != nil {
		return e
	}
	n.ended = true
	return nil
}
func (n *BitVectorSignExtensionNumerals) CloseExpression() error {
	if n == nil || n.self != n {
		return ErrASTHandle
	}
	if n.closed || n.expressionClosed {
		return nil
	}
	if e := n.manager.CleanupBitVectorNumeralInvocation(&n.call); e != nil {
		return e
	}
	if e := n.manager.CleanupBitVectorNumeralInvocation(&n.norm); e != nil {
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
func (n *BitVectorSignExtensionNumerals) Close() error {
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
