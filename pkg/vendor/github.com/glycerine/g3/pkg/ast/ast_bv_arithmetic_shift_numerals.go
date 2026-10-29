package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:891-985; src/util/util.h:131-139.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorArithmeticShiftNumerals retains r1/r2, thresholds and reached shift/padding values.
type BitVectorArithmeticShiftNumerals struct {
	self                   *BitVectorArithmeticShiftNumerals
	manager                *ASTManager
	values                 [6]unumeral.OwnedRational
	placed                 [6]bool
	scalar                 unumeral.RationalServiceInvocation
	call                   ASTBitVectorNumeralInvocation
	sign                   ASTBitVectorNumeralInvocation
	header                 uint64
	sourceCause            error
	stopped, ended, closed bool
}

func NewBitVectorArithmeticShiftNumerals(m *ASTManager) (*BitVectorArithmeticShiftNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorArithmeticShiftNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorArithmeticShiftNumerals{manager: m, header: bytes}
	n.self = n
	for _, i := range []int{0, 1} {
		if e := n.Place(i); e != nil {
			return n, e
		}
	}
	return n, nil
}

func (n *BitVectorArithmeticShiftNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorArithmeticShiftNumerals) slot(i int) error {
	if e := n.open(); e != nil {
		return e
	}
	if i < 0 || i >= len(n.values) || !n.placed[i] {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorArithmeticShiftNumerals) fail(e error) error {
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorArithmeticShiftNumerals) Place(i int) error {
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
func (n *BitVectorArithmeticShiftNumerals) retire() error {
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
func (n *BitVectorArithmeticShiftNumerals) Read(i int, arg ASTHandle, width *uint32) (bool, error) {
	if e := n.slot(i); e != nil {
		return false, e
	}
	yes, e := n.manager.BitVectorNumeralValueWithInvocation(arg, &n.values[i], width, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return yes, n.fail(e)
}

func (n *BitVectorArithmeticShiftNumerals) ZeroAmount() (bool, error) {
	if e := n.slot(1); e != nil {
		return false, e
	}
	yes, e := n.manager.numerals.IsZero(&n.values[1])
	return yes, n.fail(e)
}
func (n *BitVectorArithmeticShiftNumerals) setLiteral(i int, value uint32) error {
	if e := n.Place(i); e != nil {
		return e
	}
	e := n.manager.numerals.SetUnsignedWithInvocation(&n.values[i], value, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return n.fail(e)
}

func (n *BitVectorArithmeticShiftNumerals) setSmallLiteral(i int, value int64) error {
	if e := n.Place(i); e != nil {
		return e
	}
	e := n.manager.numerals.SetInt64WithInvocation(&n.values[i], value, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return n.fail(e)
}

// These separate comparison expressions construct/destroy the original
// numeral(bv_size), including the second reached DEBUG assertion expression.
func (n *BitVectorArithmeticShiftNumerals) AtLeastWidth(width uint32) (bool, error) {
	if e := n.slot(1); e != nil {
		return false, e
	}
	if e := n.setLiteral(2, width); e != nil {
		return false, e
	}
	yes, e := n.manager.numerals.LessEqual(&n.values[2], &n.values[1])
	if e != nil {
		return false, n.fail(e)
	}
	return yes, n.End(2)
}
func (n *BitVectorArithmeticShiftNumerals) Narrow(width uint32) error {
	if e := n.slot(0); e != nil {
		return e
	}
	q := n.manager.numerals
	a, e := q.Uint64(&n.values[0])
	if e != nil {
		return n.fail(e)
	}
	amount, e := q.Uint64(&n.values[1])
	if e != nil {
		return n.fail(e)
	}
	if width == 0 {
		return n.fail(ErrASTDomain)
	}
	reduced := amount % uint64(width)
	if ASTSourceDebug && reduced >= uint64(width) {
		return n.fail(ErrASTDomain)
	}
	r := a >> reduced
	sign := a&(uint64(1)<<(width-1)) != 0
	allone := (uint64(1) << width) - 1
	if amount > reduced {
		if sign {
			r = allone
		} else {
			r = 0
		}
	} else if sign {
		mask := ^((uint64(1) << (uint64(width) - reduced)) - 1)
		mask &= allone
		r |= mask
	}
	if e = n.Place(3); e != nil {
		return e
	}
	e = q.SetUint64WithInvocation(&n.values[3], r, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return n.fail(e)
}
func (n *BitVectorArithmeticShiftNumerals) HasSign(width uint32) (bool, error) {
	if e := n.slot(0); e != nil {
		return false, e
	}
	yes, e := n.manager.BitVectorHasSignBitWithInvocation(&n.values[0], width, &n.sign)
	return yes, n.fail(e)
}
func (n *BitVectorArithmeticShiftNumerals) AllOnes(width uint32) error {
	if e := n.open(); e != nil {
		return e
	}
	q := n.manager.numerals
	v, e := q.PowerOfTwoWithInvocation(width, &n.scalar)
	n.values[3] = v
	n.placed[3] = v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	if e = n.setSmallLiteral(4, 1); e != nil {
		return e
	}
	v, e = q.SubWithInvocation(&n.values[3], &n.values[4], &n.scalar)
	n.values[5] = v
	n.placed[5] = v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	return n.fail(e)
}
func (n *BitVectorArithmeticShiftNumerals) Wide(width uint32) error {
	if e := n.slot(0); e != nil {
		return e
	}
	if ASTSourceDebug {
		ge, e := n.AtLeastWidth(width)
		if e != nil {
			return e
		}
		if ge {
			return n.fail(ErrASTDomain)
		}
	}
	sign, e := n.HasSign(width)
	if e != nil {
		return e
	}
	q := n.manager.numerals
	amount, e := q.Unsigned(&n.values[1])
	if e != nil {
		return n.fail(e)
	}
	v, e := q.PowerOfTwoWithInvocation(amount, &n.scalar)
	n.values[3] = v
	n.placed[3] = v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	e = q.IDivIntoWithInvocation(&n.values[0], &n.values[3], &n.values[0], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	if e = n.End(3); e != nil {
		return e
	}
	if !sign {
		return nil
	}
	// Preserve the original physical p and every reached numeral(2) temporary.
	if e = n.setSmallLiteral(4, 1); e != nil {
		return e
	}
	for i := uint32(0); i < width; i++ {
		less, e := q.Less(&n.values[0], &n.values[4])
		if e != nil {
			return n.fail(e)
		}
		if less {
			e = q.AddAssignWithInvocation(&n.values[0], &n.values[4], &n.scalar)
			if e == nil {
				e = n.retire()
			}
			if e != nil {
				return n.fail(e)
			}
		}
		if e = n.setSmallLiteral(5, 2); e != nil {
			return e
		}
		e = q.MulAssignWithInvocation(&n.values[4], &n.values[5], &n.scalar)
		if e == nil {
			e = n.retire()
		}
		if e != nil {
			return n.fail(e)
		}
		if e = n.End(5); e != nil {
			return e
		}
	}
	// p's if-block ends before the final numeral factory, even on the last bit.
	return n.End(4)
}
func (n *BitVectorArithmeticShiftNumerals) CombineAndClamp(width uint32) error {
	if e := n.slot(0); e != nil {
		return e
	}
	q := n.manager.numerals
	e := q.AddAssignWithInvocation(&n.values[0], &n.values[1], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	if e = n.setLiteral(2, width); e != nil {
		return e
	}
	greater, e := q.Greater(&n.values[0], &n.values[2])
	if e != nil {
		return n.fail(e)
	}
	if e = n.End(2); e != nil {
		return e
	}
	if !greater {
		return nil
	}
	if e = n.setLiteral(2, width); e != nil {
		return e
	}
	e = q.CopyWithInvocation(&n.values[0], &n.values[2], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	return n.End(2)
}
func (n *BitVectorArithmeticShiftNumerals) Zero(width uint32) (node ASTHandle, result error) {
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
func (n *BitVectorArithmeticShiftNumerals) Constant(i int, width uint32) (node ASTHandle, result error) {
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
func (n *BitVectorArithmeticShiftNumerals) End(indices ...int) error {
	if e := n.open(); e != nil {
		return e
	}
	e := n.scalar.SourceReady()
	if e == nil {
		e = n.manager.BitVectorNumeralInvocationSourceReady(&n.call)
		if e == nil {
			e = n.manager.BitVectorNumeralInvocationSourceReady(&n.sign)
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
func (n *BitVectorArithmeticShiftNumerals) SourceEnd() error {
	if e := n.End(5, 4, 3, 2, 1, 0); e != nil {
		return e
	}
	n.ended = true
	return nil
}
func (n *BitVectorArithmeticShiftNumerals) Close() error {
	if n == nil || n.self != n {
		return ErrASTHandle
	}
	if n.closed {
		return nil
	}
	if e := n.manager.CleanupBitVectorNumeralInvocation(&n.call); e != nil {
		return e
	}
	if e := n.manager.CleanupBitVectorNumeralInvocation(&n.sign); e != nil {
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
