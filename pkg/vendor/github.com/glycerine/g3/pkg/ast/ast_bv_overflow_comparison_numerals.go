package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:262-263,298-303,315-346.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorOverflowComparisonNumerals owns a0/b0, reached difference or
// lower/upper returns and their power/subtraction expression temporaries.
type BitVectorOverflowComparisonNumerals struct {
	self                   *BitVectorOverflowComparisonNumerals
	manager                *ASTManager
	values                 [6]unumeral.OwnedRational
	placed                 [6]bool
	scalar                 unumeral.RationalServiceInvocation
	call                   ASTBitVectorNumeralInvocation
	header                 uint64
	sourceCause            error
	stopped, ended, closed bool
	expressionClosed       bool
}

func NewBitVectorOverflowComparisonNumerals(m *ASTManager) (*BitVectorOverflowComparisonNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorOverflowComparisonNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorOverflowComparisonNumerals{manager: m, header: bytes}
	n.self = n
	for i := 0; i < 2; i++ {
		v, e := m.numerals.NewValue()
		if e != nil {
			return n, n.fail(e)
		}
		n.values[i], n.placed[i] = v, true
	}
	return n, nil
}
func (n *BitVectorOverflowComparisonNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended || n.expressionClosed {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorOverflowComparisonNumerals) fail(e error) error {
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorOverflowComparisonNumerals) retire() error {
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
func (n *BitVectorOverflowComparisonNumerals) Read(i int, arg ASTHandle, width *uint32) (bool, error) {
	if e := n.open(); e != nil {
		return false, e
	}
	if i < 0 || i > 1 {
		return false, ErrASTHandle
	}
	yes, e := n.manager.BitVectorNumeralValueWithInvocation(arg, &n.values[i], width, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return yes, n.fail(e)
}

func (n *BitVectorOverflowComparisonNumerals) Zero(i int) error {
	if e := n.open(); e != nil {
		return e
	}
	if i < 0 || i > 1 {
		return ErrASTHandle
	}
	e := n.manager.numerals.CopyZeroWithInvocation(&n.values[i], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return n.fail(e)
}
func (n *BitVectorOverflowComparisonNumerals) slot(i int) error {
	if e := n.open(); e != nil {
		return e
	}
	if i < 0 || i >= len(n.values) || !n.placed[i] {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorOverflowComparisonNumerals) Equal(i, j int) (bool, error) {
	if e := n.slot(i); e != nil {
		return false, e
	}
	if e := n.slot(j); e != nil {
		return false, e
	}
	v, e := n.manager.numerals.Equal(&n.values[i], &n.values[j])
	return v, n.fail(e)
}
func (n *BitVectorOverflowComparisonNumerals) Less(i, j int) (bool, error) {
	if e := n.slot(i); e != nil {
		return false, e
	}
	if e := n.slot(j); e != nil {
		return false, e
	}
	v, e := n.manager.numerals.Less(&n.values[i], &n.values[j])
	return v, n.fail(e)
}
func (n *BitVectorOverflowComparisonNumerals) IsZero(i int) (bool, error) {
	if e := n.slot(i); e != nil {
		return false, e
	}
	v, e := n.manager.numerals.IsZero(&n.values[i])
	return v, n.fail(e)
}
func (n *BitVectorOverflowComparisonNumerals) IsNonnegative(i int) (bool, error) {
	if e := n.slot(i); e != nil {
		return false, e
	}
	v, e := n.manager.numerals.IsNeg(&n.values[i])
	return !v, n.fail(e)
}
func (n *BitVectorOverflowComparisonNumerals) IsPositive(i int) (bool, error) {
	if e := n.slot(i); e != nil {
		return false, e
	}
	v, e := n.manager.numerals.IsPos(&n.values[i])
	return v, n.fail(e)
}
func (n *BitVectorOverflowComparisonNumerals) Difference() error {
	return n.subtract(2, 1, 0)
}
func (n *BitVectorOverflowComparisonNumerals) subtract(dst, a, b int) error {
	if e := n.open(); e != nil {
		return e
	}
	if n.placed[dst] {
		return ErrASTPending
	}
	v, e := n.manager.numerals.SubWithInvocation(&n.values[a], &n.values[b], &n.scalar)
	n.values[dst], n.placed[dst] = v, v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	return n.fail(e)
}
func (n *BitVectorOverflowComparisonNumerals) power(width uint32) error {
	if e := n.open(); e != nil {
		return e
	}
	if n.placed[4] {
		return ErrASTPending
	}
	v, e := n.manager.numerals.PowerOfTwoWithInvocation(width, &n.scalar)
	n.values[4], n.placed[4] = v, v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	return n.fail(e)
}
func (n *BitVectorOverflowComparisonNumerals) Bounds(width uint32) error {
	if e := n.power(width); e != nil {
		return e
	}
	if e := n.subtract(2, 4, 0); e != nil {
		return e
	}
	// The power expression ends after initialization of const lower.
	if e := n.End(4); e != nil {
		return e
	}
	if e := n.power(width); e != nil {
		return e
	}
	if e := n.subtract(5, 4, 1); e != nil {
		return e
	}
	v, e := n.manager.numerals.SubOneWithInvocation(&n.values[5], &n.scalar)
	n.values[3], n.placed[3] = v, v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	return n.End(5, 4)
}

func (n *BitVectorOverflowComparisonNumerals) Constant(i int, width uint32) (node ASTHandle, result error) {
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
func (n *BitVectorOverflowComparisonNumerals) End(indices ...int) error {
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
func (n *BitVectorOverflowComparisonNumerals) SourceEnd() error {
	if e := n.End(5, 4, 3, 2, 1, 0); e != nil {
		return e
	}
	n.ended = true
	return nil
}
func (n *BitVectorOverflowComparisonNumerals) CloseExpression() error {
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
	for i := len(n.values) - 1; i >= 4; i-- {
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
func (n *BitVectorOverflowComparisonNumerals) Close() error {
	if n == nil || n.self != n {
		return ErrASTHandle
	}
	if n.closed {
		return nil
	}
	if e := n.CloseExpression(); e != nil {
		return e
	}
	for i := 3; i >= 0; i-- {
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
