package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:358-393.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorConcatComparisonNumerals owns af/bf, retained normalized high
// prefixes and only the reached division/power expression temporaries.
type BitVectorConcatComparisonNumerals struct {
	self                   *BitVectorConcatComparisonNumerals
	manager                *ASTManager
	values                 [6]unumeral.OwnedRational
	placed                 [6]bool
	scalar                 unumeral.RationalServiceInvocation
	call                   ASTBitVectorNumeralInvocation
	norm                   ASTBitVectorNumeralInvocation
	header                 uint64
	sourceCause            error
	stopped, ended, closed bool
	expressionClosed       bool
}

func NewBitVectorConcatComparisonNumerals(m *ASTManager) (*BitVectorConcatComparisonNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorConcatComparisonNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorConcatComparisonNumerals{manager: m, header: bytes}
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
func (n *BitVectorConcatComparisonNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended || n.expressionClosed {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorConcatComparisonNumerals) fail(e error) error {
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorConcatComparisonNumerals) retire() error {
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
func (n *BitVectorConcatComparisonNumerals) Read(i int, arg ASTHandle, width *uint32) (bool, error) {
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

func (n *BitVectorConcatComparisonNumerals) High(i int, width, commonWidth uint32, signed bool) error {
	if e := n.open(); e != nil {
		return e
	}
	if i < 0 || i > 1 || n.placed[2+i] || n.placed[4] || n.placed[5] {
		return ErrASTPending
	}
	q := n.manager.numerals
	input := i
	if width > commonWidth {
		v, e := q.PowerOfTwoWithInvocation(width-commonWidth, &n.scalar)
		n.values[4], n.placed[4] = v, v != (unumeral.OwnedRational{})
		if e == nil {
			e = n.retire()
		}
		if e != nil {
			return n.fail(e)
		}
		v, e = q.IDivWithInvocation(&n.values[i], &n.values[4], &n.scalar)
		n.values[5], n.placed[5] = v, v != (unumeral.OwnedRational{})
		if e == nil {
			e = n.retire()
		}
		if e != nil {
			return n.fail(e)
		}
		input = 5
	}
	var v unumeral.OwnedRational
	var e error
	if signed {
		v, e = n.manager.NormalizeSignedBitVectorValueWithInvocation(&n.values[input], commonWidth, &n.norm)
	} else {
		v, e = n.manager.NormalizeBitVectorValueWithInvocation(&n.values[input], commonWidth, &n.norm)
	}
	n.values[2+i], n.placed[2+i] = v, v != (unumeral.OwnedRational{})
	if e != nil {
		return n.fail(e)
	}
	// The division and power expression ends after initialization of hi_af/hi_bf.
	return n.End(5, 4)
}
func (n *BitVectorConcatComparisonNumerals) CompareHigh() (equal, less bool, err error) {
	if e := n.open(); e != nil {
		return false, false, e
	}
	equal, err = n.manager.numerals.Equal(&n.values[2], &n.values[3])
	if err == nil && !equal {
		less, err = n.manager.numerals.Less(&n.values[2], &n.values[3])
	}
	return equal, less, n.fail(err)
}

func (n *BitVectorConcatComparisonNumerals) Constant(i int, width uint32) (node ASTHandle, result error) {
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
func (n *BitVectorConcatComparisonNumerals) End(indices ...int) error {
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
func (n *BitVectorConcatComparisonNumerals) SourceEnd() error {
	if e := n.End(5, 4, 3, 2, 1, 0); e != nil {
		return e
	}
	n.ended = true
	return nil
}
func (n *BitVectorConcatComparisonNumerals) CloseExpression() error {
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
func (n *BitVectorConcatComparisonNumerals) Close() error {
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
