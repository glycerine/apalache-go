package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:1452-1469.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorConcatNumerals retains v1, v2 and the returned power-of-two temporary.
type BitVectorConcatNumerals struct {
	self                   *BitVectorConcatNumerals
	manager                *ASTManager
	values                 [3]unumeral.OwnedRational
	placed                 [3]bool
	scalar                 unumeral.RationalServiceInvocation
	call                   ASTBitVectorNumeralInvocation
	header                 uint64
	sourceCause            error
	stopped, ended, closed bool
}

func NewBitVectorConcatNumerals(m *ASTManager) (*BitVectorConcatNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorConcatNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorConcatNumerals{manager: m, header: bytes}
	n.self = n
	for _, i := range []int{0, 1} {
		if e := n.Place(i); e != nil {
			return n, e
		}
	}
	return n, nil
}

func (n *BitVectorConcatNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorConcatNumerals) slot(i int) error {
	if e := n.open(); e != nil {
		return e
	}
	if i < 0 || i >= len(n.values) || !n.placed[i] {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorConcatNumerals) fail(e error) error {
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorConcatNumerals) Place(i int) error {
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
func (n *BitVectorConcatNumerals) retire() error {
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
func (n *BitVectorConcatNumerals) Read(i int, arg ASTHandle, width *uint32) (bool, error) {
	if e := n.slot(i); e != nil {
		return false, e
	}
	yes, e := n.manager.BitVectorNumeralValueWithInvocation(arg, &n.values[i], width, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return yes, n.fail(e)
}

// Fuse modifies the physical v2 receiver. The returned power survives through
// multiplication, then ends at that full expression before v2 += v1.
func (n *BitVectorConcatNumerals) Fuse(width uint32) error {
	if e := n.slot(1); e != nil {
		return e
	}
	if n.placed[2] {
		return ErrASTPending
	}
	v, e := n.manager.numerals.PowerOfTwoWithInvocation(width, &n.scalar)
	n.values[2] = v
	n.placed[2] = v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	e = n.manager.numerals.MulAssignWithInvocation(&n.values[1], &n.values[2], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	if e = n.End(2); e != nil {
		return e
	}
	e = n.manager.numerals.AddAssignWithInvocation(&n.values[1], &n.values[0], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return n.fail(e)
}
func (n *BitVectorConcatNumerals) Constant(i int, width uint32) (node ASTHandle, result error) {
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
func (n *BitVectorConcatNumerals) End(indices ...int) error {
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
func (n *BitVectorConcatNumerals) SourceEnd() error {
	if e := n.End(2, 1, 0); e != nil {
		return e
	}
	n.ended = true
	return nil
}
func (n *BitVectorConcatNumerals) Close() error {
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
