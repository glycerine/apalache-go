package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:1599-1734,1772-1903.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorBitwiseNumerals retains v1/v2 and the reached full expressions of
// the OR/XOR bodies. Slots are v1, v2, two, power, literal one, all-one return,
// bitwise return, normalization return and modulo return. The mask's two is
// older than its repeated all-one expressions; it survives their source ends.
type BitVectorBitwiseNumerals struct {
	self                   *BitVectorBitwiseNumerals
	manager                *ASTManager
	values                 [9]unumeral.OwnedRational
	placed                 [9]bool
	scalar                 unumeral.RationalServiceInvocation
	norm, call             ASTBitVectorNumeralInvocation
	header                 uint64
	sourceCause            error
	stopped, ended, closed bool
	expressionClosed       bool
}

func NewBitVectorBitwiseNumerals(m *ASTManager) (*BitVectorBitwiseNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorBitwiseNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorBitwiseNumerals{manager: m, header: bytes}
	n.self = n
	for _, i := range []int{0, 1} {
		if e := n.place(i); e != nil {
			return n, e
		}
	}
	return n, nil
}
func (n *BitVectorBitwiseNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended || n.expressionClosed {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorBitwiseNumerals) slot(i int) error {
	if e := n.open(); e != nil {
		return e
	}
	if i < 0 || i >= len(n.values) || !n.placed[i] {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorBitwiseNumerals) fail(e error) error {
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorBitwiseNumerals) place(i int) error {
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
	n.values[i], n.placed[i] = v, true
	return nil
}
func (n *BitVectorBitwiseNumerals) retire() error {
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
func (n *BitVectorBitwiseNumerals) store(i int, v unumeral.OwnedRational, e error) error {
	n.values[i], n.placed[i] = v, v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	return n.fail(e)
}
func (n *BitVectorBitwiseNumerals) copy(dst, src int) error {
	e := n.manager.numerals.CopyWithInvocation(&n.values[dst], &n.values[src], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return n.fail(e)
}
func (n *BitVectorBitwiseNumerals) Read(arg ASTHandle, width *uint32) (bool, error) {
	if e := n.slot(1); e != nil {
		return false, e
	}
	yes, e := n.manager.BitVectorNumeralValueWithInvocation(arg, &n.values[1], width, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return yes, n.fail(e)
}

// combine assigns the distinct returned rational into physical v1 before its
// full-expression destruction. Neither stored input nor v1 is normalized here.
func (n *BitVectorBitwiseNumerals) combine(input int, xor bool) error {
	if e := n.slot(0); e != nil {
		return e
	}
	if e := n.slot(input); e != nil {
		return e
	}
	if n.placed[6] {
		return ErrASTPending
	}
	q := n.manager.numerals
	var v unumeral.OwnedRational
	var e error
	if xor {
		v, e = q.BitwiseXorWithInvocation(&n.values[0], &n.values[input], &n.scalar)
	} else {
		v, e = q.BitwiseOrWithInvocation(&n.values[0], &n.values[input], &n.scalar)
	}
	if e = n.store(6, v, e); e != nil {
		return e
	}
	if e = n.copy(0, 6); e != nil {
		return e
	}
	return n.End(6)
}
func (n *BitVectorBitwiseNumerals) Combine(xor bool) error { return n.combine(1, xor) }
func (n *BitVectorBitwiseNumerals) IsZero() (bool, error) {
	if e := n.slot(0); e != nil {
		return false, e
	}
	yes, e := n.manager.numerals.IsZero(&n.values[0])
	return yes, n.fail(e)
}

// AllOne evaluates power_of_two(width) - numeral(1). Its three temporaries
// survive until the caller ends the comparison/assignment full expression.
func (n *BitVectorBitwiseNumerals) AllOne(width uint32) error {
	if e := n.open(); e != nil {
		return e
	}
	for _, i := range []int{3, 4, 5} {
		if n.placed[i] {
			return ErrASTPending
		}
	}
	q := n.manager.numerals
	v, e := q.PowerOfTwoWithInvocation(width, &n.scalar)
	if e = n.store(3, v, e); e != nil {
		return e
	}
	if e = n.place(4); e != nil {
		return e
	}
	e = q.SetInt64WithInvocation(&n.values[4], 1, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	v, e = q.SubWithInvocation(&n.values[3], &n.values[4], &n.scalar)
	return n.store(5, v, e)
}
func (n *BitVectorBitwiseNumerals) IsAllOne(width uint32) (bool, error) {
	if e := n.slot(0); e != nil {
		return false, e
	}
	if e := n.AllOne(width); e != nil {
		return false, e
	}
	yes, e := n.manager.numerals.Equal(&n.values[0], &n.values[5])
	if e != nil {
		return false, n.fail(e)
	}
	return yes, n.End(5, 4, 3)
}
func (n *BitVectorBitwiseNumerals) ToggleAllOne(width uint32) error {
	if e := n.AllOne(width); e != nil {
		return e
	}
	if e := n.combine(5, true); e != nil {
		return e
	}
	return n.End(5, 4, 3)
}
func (n *BitVectorBitwiseNumerals) Normalize(width uint32) error {
	if e := n.slot(0); e != nil {
		return e
	}
	if n.placed[7] {
		return ErrASTPending
	}
	v, e := n.manager.NormalizeBitVectorValueWithInvocation(&n.values[0], width, &n.norm)
	n.values[7], n.placed[7] = v, v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.manager.BitVectorNumeralInvocationSourceReady(&n.norm)
	}
	if e != nil {
		return n.fail(e)
	}
	if e = n.copy(0, 7); e != nil {
		return e
	}
	return n.End(7)
}

// MaskValueString snapshots the normalized coefficient before the mask loop shifts it.
func (n *BitVectorBitwiseNumerals) MaskValueString() (string, error) {
	if e := n.slot(0); e != nil {
		return "", e
	}
	value, e := n.manager.numerals.String(&n.values[0])
	return value, n.fail(e)
}
func (n *BitVectorBitwiseNumerals) BeginMask() error {
	if e := n.place(2); e != nil {
		return e
	}
	e := n.manager.numerals.SetInt64WithInvocation(&n.values[2], 2, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return n.fail(e)
}

// LowBit preserves each separate mod(v1,two).is_one/is_zero condition and
// destroys its returned rational before the loop body increments/divides.
func (n *BitVectorBitwiseNumerals) LowBit(one bool) (bool, error) {
	if e := n.slot(0); e != nil {
		return false, e
	}
	if e := n.slot(2); e != nil {
		return false, e
	}
	if n.placed[8] {
		return false, ErrASTPending
	}
	q := n.manager.numerals
	v, e := q.ModWithInvocation(&n.values[0], &n.values[2], &n.scalar)
	if e = n.store(8, v, e); e != nil {
		return false, e
	}
	var yes bool
	if one {
		yes, e = q.IsOne(&n.values[8])
	} else {
		yes, e = q.IsZero(&n.values[8])
	}
	if e != nil {
		return false, n.fail(e)
	}
	return yes, n.End(8)
}
func (n *BitVectorBitwiseNumerals) ShiftMask() error {
	if e := n.slot(0); e != nil {
		return e
	}
	if e := n.slot(2); e != nil {
		return e
	}
	e := n.manager.numerals.IDivIntoWithInvocation(&n.values[0], &n.values[2], &n.values[0], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return n.fail(e)
}
func (n *BitVectorBitwiseNumerals) Constant(i int, width uint32) (node ASTHandle, result error) {
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
func (n *BitVectorBitwiseNumerals) End(indices ...int) error {
	if e := n.open(); e != nil {
		return e
	}
	e := n.scalar.SourceReady()
	if e == nil {
		e = n.manager.BitVectorNumeralInvocationSourceReady(&n.call)
	}
	if e == nil {
		e = n.manager.BitVectorNumeralInvocationSourceReady(&n.norm)
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
func (n *BitVectorBitwiseNumerals) SourceEnd() error {
	if e := n.End(8, 7, 6, 5, 4, 3, 2, 1, 0); e != nil {
		return e
	}
	n.ended = true
	return nil
}

// CloseExpression joins younger numerical children/temporaries before the
// caller releases its borrowed mask buffer. The older two/v2/v1 remain placed.
func (n *BitVectorBitwiseNumerals) CloseExpression() error {
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
	for i := len(n.values) - 1; i >= 3; i-- {
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
func (n *BitVectorBitwiseNumerals) Close() error {
	if n == nil || n.self != n {
		return ErrASTHandle
	}
	if n.closed {
		return nil
	}
	if e := n.CloseExpression(); e != nil {
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
