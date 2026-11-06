package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:436-517,560-568.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorLEComparisonNumerals retains r1/r2 and the original unused r3,
// later default lower/upper, and only reached expression temporaries. Slots
// 5..11 are power, signed literal, negation, arithmetic return, norm return,
// remainder return and sum return respectively.
type BitVectorLEComparisonNumerals struct {
	self                   *BitVectorLEComparisonNumerals
	manager                *ASTManager
	values                 [12]unumeral.OwnedRational
	placed                 [12]bool
	scalar                 unumeral.RationalServiceInvocation
	call                   ASTBitVectorNumeralInvocation
	norm                   ASTBitVectorNumeralInvocation
	header                 uint64
	sourceCause            error
	stopped, ended, closed bool
	expressionClosed       bool
	tempOrder              [12]uint64
	sequence               uint64
}

func NewBitVectorLEComparisonNumerals(m *ASTManager) (*BitVectorLEComparisonNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorLEComparisonNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorLEComparisonNumerals{manager: m, header: bytes}
	n.self = n
	for i := 0; i < 3; i++ {
		v, e := m.numerals.NewValue()
		if e != nil {
			return n, n.fail(e)
		}
		n.values[i], n.placed[i] = v, true
	}
	return n, nil
}
func (n *BitVectorLEComparisonNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended || n.expressionClosed {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorLEComparisonNumerals) fail(e error) error {
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorLEComparisonNumerals) retire() error {
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
func (n *BitVectorLEComparisonNumerals) Read(i int, arg ASTHandle, width *uint32) (bool, error) {
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

func (n *BitVectorLEComparisonNumerals) slot(i int) error {
	if e := n.open(); e != nil {
		return e
	}
	if i < 0 || i >= len(n.values) || !n.placed[i] {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorLEComparisonNumerals) Equal(i, j int) (bool, error) {
	if e := n.slot(i); e != nil {
		return false, e
	}
	if e := n.slot(j); e != nil {
		return false, e
	}
	v, e := n.manager.numerals.Equal(&n.values[i], &n.values[j])
	return v, n.fail(e)
}

func (n *BitVectorLEComparisonNumerals) IsPositive(i int) (bool, error) {
	if e := n.slot(i); e != nil {
		return false, e
	}
	v, e := n.manager.numerals.IsPos(&n.values[i])
	return v, n.fail(e)
}
func (n *BitVectorLEComparisonNumerals) store(i int, v unumeral.OwnedRational, e error) error {
	n.values[i], n.placed[i] = v, v != (unumeral.OwnedRational{})
	n.recordTemp(i)
	if e == nil {
		e = n.retire()
	}
	return n.fail(e)
}
func (n *BitVectorLEComparisonNumerals) copy(dst, src int) error {
	e := n.manager.numerals.CopyWithInvocation(&n.values[dst], &n.values[src], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return n.fail(e)
}
func (n *BitVectorLEComparisonNumerals) place(i int) error {
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
	n.recordTemp(i)
	return nil
}
func (n *BitVectorLEComparisonNumerals) literal(value int64) error {
	if e := n.place(6); e != nil {
		return e
	}
	e := n.manager.numerals.SetInt64WithInvocation(&n.values[6], value, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return n.fail(e)
}
func (n *BitVectorLEComparisonNumerals) power(width uint32) error {
	if e := n.open(); e != nil {
		return e
	}
	if n.placed[5] {
		return ErrASTPending
	}
	v, e := n.manager.numerals.PowerOfTwoWithInvocation(width, &n.scalar)
	return n.store(5, v, e)
}
func (n *BitVectorLEComparisonNumerals) normValue(i int, width uint32, signed bool) error {
	if e := n.slot(i); e != nil {
		return e
	}
	if n.placed[9] {
		return ErrASTPending
	}
	var v unumeral.OwnedRational
	var e error
	if signed {
		v, e = n.manager.NormalizeSignedBitVectorValueWithInvocation(&n.values[i], width, &n.norm)
	} else {
		v, e = n.manager.NormalizeBitVectorValueWithInvocation(&n.values[i], width, &n.norm)
	}
	n.values[9], n.placed[9] = v, v != (unumeral.OwnedRational{})
	n.recordTemp(9)
	if e == nil {
		e = n.manager.BitVectorNumeralInvocationSourceReady(&n.norm)
	}
	return n.fail(e)
}
func (n *BitVectorLEComparisonNumerals) Normalize(i int, width uint32, signed bool) error {
	if e := n.normValue(i, width, signed); e != nil {
		return e
	}
	if e := n.copy(i, 9); e != nil {
		return e
	}
	return n.End(9)
}
func (n *BitVectorLEComparisonNumerals) MinusOneNormalized(width uint32, signed bool) (bool, error) {
	if e := n.normValue(0, width, signed); e != nil {
		return false, e
	}
	yes, e := n.manager.numerals.IsMinusOne(&n.values[9])
	if e != nil {
		return false, n.fail(e)
	}
	return yes, n.End(9)
}
func (n *BitVectorLEComparisonNumerals) CompareOperands() (bool, error) {
	if e := n.slot(0); e != nil {
		return false, e
	}
	if e := n.slot(1); e != nil {
		return false, e
	}
	yes, e := n.manager.numerals.LessEqual(&n.values[0], &n.values[1])
	return yes, n.fail(e)
}

// PlaceBounds preserves the unconditional later default lower then upper.
func (n *BitVectorLEComparisonNumerals) PlaceBounds() error {
	if e := n.place(3); e != nil {
		return e
	}
	return n.place(4)
}
func (n *BitVectorLEComparisonNumerals) Bounds(width uint32, signed bool) error {
	if e := n.slot(3); e != nil {
		return e
	}
	if e := n.slot(4); e != nil {
		return e
	}
	q := n.manager.numerals
	if signed {
		if e := n.power(width - 1); e != nil {
			return e
		}
		v, e := q.NegWithInvocation(&n.values[5], &n.scalar)
		if e = n.store(7, v, e); e != nil {
			return e
		}
		if e = n.copy(3, 7); e != nil {
			return e
		}
		if e = n.End(7, 5); e != nil {
			return e
		}
	} else {
		if e := n.literal(0); e != nil {
			return e
		}
		if e := n.copy(3, 6); e != nil {
			return e
		}
		if e := n.End(6); e != nil {
			return e
		}
	}
	powerWidth := width
	if signed {
		powerWidth = width - 1
	}
	if e := n.power(powerWidth); e != nil {
		return e
	}
	if e := n.literal(1); e != nil {
		return e
	}
	v, e := q.SubWithInvocation(&n.values[5], &n.values[6], &n.scalar)
	if e = n.store(8, v, e); e != nil {
		return e
	}
	if e = n.copy(4, 8); e != nil {
		return e
	}
	return n.End(8, 6, 5)
}
func (n *BitVectorLEComparisonNumerals) RemainderIsZero() (bool, error) {
	if e := n.open(); e != nil {
		return false, e
	}
	if n.placed[10] {
		return false, ErrASTPending
	}
	v, e := n.manager.numerals.RemWithInvocation(&n.values[1], &n.values[0], &n.scalar)
	if e = n.store(10, v, e); e != nil {
		return false, e
	}
	zero, e := n.manager.numerals.IsZero(&n.values[10])
	if e != nil {
		return false, n.fail(e)
	}
	return zero, n.End(10)
}
func (n *BitVectorLEComparisonNumerals) SumMinusOne() error {
	if e := n.open(); e != nil {
		return e
	}
	if n.placed[11] || n.placed[8] {
		return ErrASTPending
	}
	q := n.manager.numerals
	v, e := q.AddWithInvocation(&n.values[0], &n.values[1], &n.scalar)
	if e = n.store(11, v, e); e != nil {
		return e
	}
	v, e = q.SubOneWithInvocation(&n.values[11], &n.scalar)
	return n.store(8, v, e)
}
func (n *BitVectorLEComparisonNumerals) SumFitsSigned(width uint32) (bool, error) {
	if e := n.SumMinusOne(); e != nil {
		return false, e
	}
	if e := n.power(width - 1); e != nil {
		return false, e
	}
	fits, e := n.manager.numerals.Less(&n.values[8], &n.values[5])
	if e != nil {
		return false, n.fail(e)
	}
	return fits, n.End(5, 8, 11)
}
func (n *BitVectorLEComparisonNumerals) ZeroConstant(width uint32) (ASTHandle, error) {
	if e := n.literal(0); e != nil {
		return ASTHandle{}, e
	}
	return n.Constant(6, width)
}

func (n *BitVectorLEComparisonNumerals) Constant(i int, width uint32) (node ASTHandle, result error) {
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
func (n *BitVectorLEComparisonNumerals) End(indices ...int) error {
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
			n.tempOrder[i] = 0
		}
	}
	return nil
}
func (n *BitVectorLEComparisonNumerals) recordTemp(i int) {
	if i >= 5 && n.placed[i] {
		n.sequence++
		n.tempOrder[i] = n.sequence
	}
}
func (n *BitVectorLEComparisonNumerals) youngestTemp() int {
	found := -1
	var stamp uint64
	for i := 5; i < len(n.values); i++ {
		if n.placed[i] && n.tempOrder[i] > stamp {
			stamp = n.tempOrder[i]
			found = i
		}
	}
	return found
}
func (n *BitVectorLEComparisonNumerals) EndExpression() error {
	if e := n.open(); e != nil {
		return e
	}
	for {
		i := n.youngestTemp()
		if i < 0 {
			return nil
		}
		if e := n.End(i); e != nil {
			return e
		}
	}
}
func (n *BitVectorLEComparisonNumerals) SourceEnd() error {
	if e := n.EndExpression(); e != nil {
		return e
	}
	if e := n.End(4, 3, 2, 1, 0); e != nil {
		return e
	}
	n.ended = true
	return nil
}
func (n *BitVectorLEComparisonNumerals) CloseExpression() error {
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
	for {
		i := n.youngestTemp()
		if i < 0 {
			break
		}
		if e := n.manager.numerals.CleanupValue(&n.values[i]); e != nil {
			return e
		}
		n.placed[i] = false
		n.tempOrder[i] = 0
	}
	n.expressionClosed = true
	return nil
}
func (n *BitVectorLEComparisonNumerals) Close() error {
	if n == nil || n.self != n {
		return ErrASTHandle
	}
	if n.closed {
		return nil
	}
	if e := n.CloseExpression(); e != nil {
		return e
	}
	for i := 4; i >= 0; i-- {
		if n.placed[i] {
			if e := n.manager.numerals.CleanupValue(&n.values[i]); e != nil {
				return e
			}
			n.placed[i] = false
			n.tempOrder[i] = 0
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
