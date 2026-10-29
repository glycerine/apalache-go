package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:987-1032; src/ast/bv_decl_plugin.cpp:753-765.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorSignedDivisionNumerals retains r1/r2 and their distinct norm/quotient/allone returns.
type BitVectorSignedDivisionNumerals struct {
	self                   *BitVectorSignedDivisionNumerals
	manager                *ASTManager
	values                 [7]unumeral.OwnedRational
	placed                 [7]bool
	scalar                 unumeral.RationalServiceInvocation
	call                   ASTBitVectorNumeralInvocation
	norm                   ASTBitVectorNumeralInvocation
	header                 uint64
	sourceCause            error
	stopped, ended, closed bool
}

func NewBitVectorSignedDivisionNumerals(m *ASTManager) (*BitVectorSignedDivisionNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorSignedDivisionNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorSignedDivisionNumerals{manager: m, header: bytes}
	n.self = n
	for _, i := range []int{0, 1} {
		if e := n.Place(i); e != nil {
			return n, e
		}
	}
	return n, nil
}

func (n *BitVectorSignedDivisionNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorSignedDivisionNumerals) slot(i int) error {
	if e := n.open(); e != nil {
		return e
	}
	if i < 0 || i >= len(n.values) || !n.placed[i] {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorSignedDivisionNumerals) fail(e error) error {
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorSignedDivisionNumerals) Place(i int) error {
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
func (n *BitVectorSignedDivisionNumerals) retire() error {
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
func (n *BitVectorSignedDivisionNumerals) Read(i int, arg ASTHandle, width *uint32) (bool, error) {
	if e := n.slot(i); e != nil {
		return false, e
	}
	yes, e := n.manager.BitVectorNumeralValueWithInvocation(arg, &n.values[i], width, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return yes, n.fail(e)
}

func (n *BitVectorSignedDivisionNumerals) Normalize(i int, width uint32) error {
	if e := n.slot(i); e != nil {
		return e
	}
	if n.placed[2] {
		return ErrASTPending
	}
	v, e := n.manager.NormalizeSignedBitVectorValueWithInvocation(&n.values[i], width, &n.norm)
	n.values[2] = v
	n.placed[2] = v != (unumeral.OwnedRational{})
	if e != nil {
		return n.fail(e)
	}
	if e = n.manager.BitVectorNumeralInvocationSourceReady(&n.norm); e != nil {
		return n.fail(e)
	}
	e = n.manager.numerals.CopyWithInvocation(&n.values[i], &n.values[2], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	return n.End(2)
}
func (n *BitVectorSignedDivisionNumerals) IsZero(i int) (bool, error) {
	if e := n.slot(i); e != nil {
		return false, e
	}
	yes, e := n.manager.numerals.IsZero(&n.values[i])
	return yes, n.fail(e)
}
func (n *BitVectorSignedDivisionNumerals) IsOne(i int) (bool, error) {
	if e := n.slot(i); e != nil {
		return false, e
	}
	yes, e := n.manager.numerals.IsOne(&n.values[i])
	return yes, n.fail(e)
}
func (n *BitVectorSignedDivisionNumerals) Quotient() error {
	if e := n.slot(0); e != nil {
		return e
	}
	if n.placed[3] {
		return ErrASTPending
	}
	// Original machine_div returns a distinct quotient truncated toward zero.
	v, e := n.manager.numerals.MachineDivWithInvocation(&n.values[0], &n.values[1], &n.scalar)
	n.values[3] = v
	n.placed[3] = v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	return n.fail(e)
}
func (n *BitVectorSignedDivisionNumerals) AllOnes(width uint32) error {
	if e := n.open(); e != nil {
		return e
	}
	q := n.manager.numerals
	v, e := q.PowerOfTwoWithInvocation(width, &n.scalar)
	n.values[4] = v
	n.placed[4] = v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	if e = n.Place(5); e != nil {
		return e
	}
	e = q.SetInt64WithInvocation(&n.values[5], 1, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	v, e = q.SubWithInvocation(&n.values[4], &n.values[5], &n.scalar)
	n.values[6] = v
	n.placed[6] = v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	return n.fail(e)
}
func (n *BitVectorSignedDivisionNumerals) Unsigned(value, width uint32) (node ASTHandle, result error) {
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
	r, e := m.bitVectorNumeralUnsignedCaptured(value, width, &n.call.receipt)
	return m.external(r), e
}
func (n *BitVectorSignedDivisionNumerals) Constant(i int, width uint32) (node ASTHandle, result error) {
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
func (n *BitVectorSignedDivisionNumerals) End(indices ...int) error {
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
func (n *BitVectorSignedDivisionNumerals) SourceEnd() error {
	if e := n.End(6, 5, 4, 3, 2, 1, 0); e != nil {
		return e
	}
	n.ended = true
	return nil
}
func (n *BitVectorSignedDivisionNumerals) Close() error {
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
