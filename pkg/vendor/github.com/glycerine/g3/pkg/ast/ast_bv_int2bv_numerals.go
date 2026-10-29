package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:1343-1361; src/ast/bv_decl_plugin.cpp:753-765.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorInt2BVNumerals retains the default val and its distinct unsigned norm return.
type BitVectorInt2BVNumerals struct {
	self                   *BitVectorInt2BVNumerals
	manager                *ASTManager
	values                 [2]unumeral.OwnedRational
	placed                 [2]bool
	scalar                 unumeral.RationalServiceInvocation
	call                   ASTBitVectorNumeralInvocation
	norm                   ASTBitVectorNumeralInvocation
	header                 uint64
	sourceCause            error
	stopped, ended, closed bool
}

func NewBitVectorInt2BVNumerals(m *ASTManager) (*BitVectorInt2BVNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorInt2BVNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorInt2BVNumerals{manager: m, header: bytes}
	n.self = n
	for _, i := range []int{0} {
		if e := n.Place(i); e != nil {
			return n, e
		}
	}
	return n, nil
}

func (n *BitVectorInt2BVNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorInt2BVNumerals) slot(i int) error {
	if e := n.open(); e != nil {
		return e
	}
	if i < 0 || i >= len(n.values) || !n.placed[i] {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorInt2BVNumerals) fail(e error) error {
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorInt2BVNumerals) Place(i int) error {
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
func (n *BitVectorInt2BVNumerals) retire() error {
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
func (n *BitVectorInt2BVNumerals) Read(family FamilyID, arg ASTHandle, integer *bool) (bool, error) {
	if e := n.slot(0); e != nil {
		return false, e
	}
	yes, e := n.manager.ArithmeticNumeralValueWithInvocation(family, arg, &n.values[0], integer, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return yes, n.fail(e)
}

func (n *BitVectorInt2BVNumerals) Normalize(width uint32) error {
	if e := n.slot(0); e != nil {
		return e
	}
	if n.placed[1] {
		return ErrASTPending
	}
	v, e := n.manager.NormalizeBitVectorValueWithInvocation(&n.values[0], width, &n.norm)
	n.values[1] = v
	n.placed[1] = v != (unumeral.OwnedRational{})
	if e != nil {
		return n.fail(e)
	}
	if e = n.manager.BitVectorNumeralInvocationSourceReady(&n.norm); e != nil {
		return n.fail(e)
	}
	e = n.manager.numerals.CopyWithInvocation(&n.values[0], &n.values[1], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	return n.End(1)
}
func (n *BitVectorInt2BVNumerals) Constant(i int, width uint32) (node ASTHandle, result error) {
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
func (n *BitVectorInt2BVNumerals) End(indices ...int) error {
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
func (n *BitVectorInt2BVNumerals) SourceEnd() error {
	if e := n.End(1, 0); e != nil {
		return e
	}
	n.ended = true
	return nil
}
func (n *BitVectorInt2BVNumerals) Close() error {
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
