package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:792-889; src/util/util.h:131-139.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorLogicalShiftNumerals retains r1/r2, threshold, rn and wide returned values.
type BitVectorLogicalShiftNumerals struct {
	self                   *BitVectorLogicalShiftNumerals
	manager                *ASTManager
	values                 [6]unumeral.OwnedRational
	placed                 [6]bool
	scalar                 unumeral.RationalServiceInvocation
	call                   ASTBitVectorNumeralInvocation
	norm                   ASTBitVectorNumeralInvocation
	header                 uint64
	sourceCause            error
	stopped, ended, closed bool
}

func NewBitVectorLogicalShiftNumerals(m *ASTManager) (*BitVectorLogicalShiftNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorLogicalShiftNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorLogicalShiftNumerals{manager: m, header: bytes}
	n.self = n
	for _, i := range []int{0, 1} {
		if e := n.Place(i); e != nil {
			return n, e
		}
	}
	return n, nil
}

func (n *BitVectorLogicalShiftNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorLogicalShiftNumerals) slot(i int) error {
	if e := n.open(); e != nil {
		return e
	}
	if i < 0 || i >= len(n.values) || !n.placed[i] {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorLogicalShiftNumerals) fail(e error) error {
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorLogicalShiftNumerals) Place(i int) error {
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
func (n *BitVectorLogicalShiftNumerals) retire() error {
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
func (n *BitVectorLogicalShiftNumerals) Read(i int, arg ASTHandle, width *uint32) (bool, error) {
	if e := n.slot(i); e != nil {
		return false, e
	}
	yes, e := n.manager.BitVectorNumeralValueWithInvocation(arg, &n.values[i], width, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return yes, n.fail(e)
}

func (n *BitVectorLogicalShiftNumerals) ZeroAmount() (bool, error) {
	if e := n.slot(1); e != nil {
		return false, e
	}
	yes, e := n.manager.numerals.IsZero(&n.values[1])
	return yes, n.fail(e)
}

// The numeral(bv_size) temporary belongs to this comparison full expression.
func (n *BitVectorLogicalShiftNumerals) AtLeastWidth(width uint32) (bool, error) {
	if e := n.slot(1); e != nil {
		return false, e
	}
	if e := n.Place(2); e != nil {
		return false, e
	}
	e := n.manager.numerals.SetUnsignedWithInvocation(&n.values[2], width, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return false, n.fail(e)
	}
	yes, e := n.manager.numerals.GreaterEqual(&n.values[1], &n.values[2])
	if e != nil {
		return false, n.fail(e)
	}
	return yes, n.End(2)
}
func (n *BitVectorLogicalShiftNumerals) Amount(width uint32, left, symbolic bool) (uint32, error) {
	if e := n.slot(1); e != nil {
		return 0, e
	}
	q := n.manager.numerals
	if ASTSourceDebug {
		if symbolic {
			yes, e := q.IsPos(&n.values[1])
			if e != nil {
				return 0, n.fail(e)
			}
			if !yes {
				return 0, n.fail(ErrASTDomain)
			}
		}
		if symbolic || left {
			ge, e := n.AtLeastWidth(width)
			if e != nil {
				return 0, e
			}
			if ge {
				return 0, n.fail(ErrASTDomain)
			}
		}
		if !symbolic || !left {
			yes, e := q.IsUnsigned(&n.values[1])
			if e != nil {
				return 0, n.fail(e)
			}
			if !yes {
				return 0, n.fail(ErrASTDomain)
			}
		}
	}
	amount, e := q.Unsigned(&n.values[1])
	return amount, n.fail(e)
}
func (n *BitVectorLogicalShiftNumerals) normalizeCopy(dst, src int, width uint32) error {
	q := n.manager.numerals
	v, e := n.manager.NormalizeBitVectorValueWithInvocation(&n.values[src], width, &n.norm)
	n.values[5] = v
	n.placed[5] = v != (unumeral.OwnedRational{})
	if e != nil {
		return n.fail(e)
	}
	if e = n.manager.BitVectorNumeralInvocationSourceReady(&n.norm); e != nil {
		return n.fail(e)
	}
	e = q.CopyWithInvocation(&n.values[dst], &n.values[5], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	return n.End(5)
}
func (n *BitVectorLogicalShiftNumerals) Narrow(width uint32, left bool) error {
	if e := n.slot(0); e != nil {
		return e
	}
	q := n.manager.numerals
	if ASTSourceDebug {
		for _, i := range []int{0, 1} {
			yes, e := q.IsUint64(&n.values[i])
			if e != nil {
				return n.fail(e)
			}
			if !yes {
				return n.fail(ErrASTDomain)
			}
		}
	}
	a, e := q.Uint64(&n.values[0])
	if e != nil {
		return n.fail(e)
	}
	b, e := q.Uint64(&n.values[1])
	if e != nil {
		return n.fail(e)
	}
	if ASTSourceDebug && left && b >= uint64(width) {
		return n.fail(ErrASTDomain)
	}
	// Go's uint64 shifts match the original saturating shift_left/shift_right.
	var value uint64
	if left {
		value = a << b
	} else {
		value = a >> b
	}
	if e = n.Place(3); e != nil {
		return e
	}
	e = q.SetUint64WithInvocation(&n.values[3], value, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	return n.normalizeCopy(3, 3, width)
}
func (n *BitVectorLogicalShiftNumerals) Wide(width uint32, left bool) error {
	if e := n.slot(0); e != nil {
		return e
	}
	shift, e := n.Amount(width, left, false)
	if e != nil {
		return e
	}
	q := n.manager.numerals
	v, e := q.PowerOfTwoWithInvocation(shift, &n.scalar)
	n.values[3] = v
	n.placed[3] = v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	if left {
		v, e = q.MulWithInvocation(&n.values[0], &n.values[3], &n.scalar)
		n.values[4] = v
		n.placed[4] = v != (unumeral.OwnedRational{})
		if e == nil {
			e = n.retire()
		}
		if e != nil {
			return n.fail(e)
		}
		if e = n.normalizeCopy(0, 4, width); e != nil {
			return e
		}
		return n.End(4, 3)
	}
	// Source uses void aliased div, with no returned/copy-normalized rational.
	e = q.IDivIntoWithInvocation(&n.values[0], &n.values[3], &n.values[0], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	return n.End(3)
}
func (n *BitVectorLogicalShiftNumerals) Zero(width uint32) (node ASTHandle, result error) {
	if e := n.open(); e != nil {
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
	r, e := m.bitVectorNumeralUnsignedCaptured(0, width, &n.call.receipt)
	return m.external(r), e
}
func (n *BitVectorLogicalShiftNumerals) Constant(i int, width uint32) (node ASTHandle, result error) {
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
func (n *BitVectorLogicalShiftNumerals) End(indices ...int) error {
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
func (n *BitVectorLogicalShiftNumerals) SourceEnd() error {
	if e := n.End(5, 4, 3, 2, 1, 0); e != nil {
		return e
	}
	n.ended = true
	return nil
}
func (n *BitVectorLogicalShiftNumerals) Close() error {
	if n == nil || n.self != n {
		return ErrASTHandle
	}
	if n.closed {
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
