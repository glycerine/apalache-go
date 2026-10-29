package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:1282-1341; src/ast/bv_decl_plugin.cpp:753-765.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorSignedModuloNumerals retains r1/r2, abs operands, u, r and reached full-expression returns.
type BitVectorSignedModuloNumerals struct {
	self                   *BitVectorSignedModuloNumerals
	manager                *ASTManager
	values                 [9]unumeral.OwnedRational
	placed                 [9]bool
	scalar                 unumeral.RationalServiceInvocation
	call                   ASTBitVectorNumeralInvocation
	norm                   ASTBitVectorNumeralInvocation
	header                 uint64
	sourceCause            error
	stopped, ended, closed bool
}

func NewBitVectorSignedModuloNumerals(m *ASTManager) (*BitVectorSignedModuloNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorSignedModuloNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorSignedModuloNumerals{manager: m, header: bytes}
	n.self = n
	for _, i := range []int{0, 1} {
		if e := n.Place(i); e != nil {
			return n, e
		}
	}
	return n, nil
}

func (n *BitVectorSignedModuloNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorSignedModuloNumerals) slot(i int) error {
	if e := n.open(); e != nil {
		return e
	}
	if i < 0 || i >= len(n.values) || !n.placed[i] {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorSignedModuloNumerals) fail(e error) error {
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorSignedModuloNumerals) Place(i int) error {
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
func (n *BitVectorSignedModuloNumerals) retire() error {
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
func (n *BitVectorSignedModuloNumerals) Read(i int, arg ASTHandle, width *uint32) (bool, error) {
	if e := n.slot(i); e != nil {
		return false, e
	}
	yes, e := n.manager.BitVectorNumeralValueWithInvocation(arg, &n.values[i], width, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return yes, n.fail(e)
}

func (n *BitVectorSignedModuloNumerals) Normalize(i int, width uint32) error {
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
func (n *BitVectorSignedModuloNumerals) IsZero(i int) (bool, error) {
	if e := n.slot(i); e != nil {
		return false, e
	}
	yes, e := n.manager.numerals.IsZero(&n.values[i])
	return yes, n.fail(e)
}
func (n *BitVectorSignedModuloNumerals) IsOne(i int) (bool, error) {
	if e := n.slot(i); e != nil {
		return false, e
	}
	yes, e := n.manager.numerals.IsOne(&n.values[i])
	return yes, n.fail(e)
}
func (n *BitVectorSignedModuloNumerals) Zero(width uint32) (node ASTHandle, result error) {
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
func (n *BitVectorSignedModuloNumerals) Constant(i int, width uint32) (node ASTHandle, result error) {
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
func (n *BitVectorSignedModuloNumerals) End(indices ...int) error {
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
func (n *BitVectorSignedModuloNumerals) SourceEnd() error {
	if e := n.End(8, 7, 6, 5, 4, 3, 2, 1, 0); e != nil {
		return e
	}
	n.ended = true
	return nil
}
func (n *BitVectorSignedModuloNumerals) Close() error {
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

func (n *BitVectorSignedModuloNumerals) store(i int, v unumeral.OwnedRational, e error) error {
	n.values[i] = v
	n.placed[i] = v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	return n.fail(e)
}
func (n *BitVectorSignedModuloNumerals) unsignedNorm(input, out int, width uint32) error {
	if n.placed[out] {
		return ErrASTPending
	}
	if e := n.slot(input); e != nil {
		return e
	}
	v, e := n.manager.NormalizeBitVectorValueWithInvocation(&n.values[input], width, &n.norm)
	n.values[out] = v
	n.placed[out] = v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.manager.BitVectorNumeralInvocationSourceReady(&n.norm)
	}
	return n.fail(e)
}
func (n *BitVectorSignedModuloNumerals) copy(dst, src int) error {
	if e := n.slot(dst); e != nil {
		return e
	}
	if e := n.slot(src); e != nil {
		return e
	}
	e := n.manager.numerals.CopyWithInvocation(&n.values[dst], &n.values[src], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return n.fail(e)
}
func (n *BitVectorSignedModuloNumerals) sign(i int, positive bool) (bool, error) {
	if e := n.slot(i); e != nil {
		return false, e
	}
	var yes bool
	var e error
	if positive {
		yes, e = n.manager.numerals.IsPos(&n.values[i])
	} else {
		yes, e = n.manager.numerals.IsNeg(&n.values[i])
	}
	return yes, n.fail(e)
}

// Modulo retains abs_r1, abs_r2, u and default r as distinct physical block
// locals. Nested arithmetic returns end at each source full expression.
func (n *BitVectorSignedModuloNumerals) Modulo(width uint32) error {
	if e := n.slot(0); e != nil {
		return e
	}
	if e := n.slot(1); e != nil {
		return e
	}
	for _, placed := range n.placed[2:] {
		if placed {
			return ErrASTPending
		}
	}
	q := n.manager.numerals
	for _, pair := range [][2]int{{0, 3}, {1, 4}} {
		v, e := q.AbsWithInvocation(&n.values[pair[0]], &n.scalar)
		if e = n.store(7, v, e); e != nil {
			return e
		}
		if e = n.unsignedNorm(7, pair[1], width); e != nil {
			return e
		}
		if e = n.End(7); e != nil {
			return e
		}
	}
	v, e := q.RemWithInvocation(&n.values[3], &n.values[4], &n.scalar)
	if e = n.store(8, v, e); e != nil {
		return e
	}
	if e = n.unsignedNorm(8, 5, width); e != nil {
		return e
	}
	if e = n.End(8); e != nil {
		return e
	}
	if e = n.Place(6); e != nil {
		return e
	}
	zero, e := n.IsZero(5)
	if e != nil {
		return e
	}
	if zero {
		return n.copy(6, 5)
	}
	positive1, e := n.sign(0, true)
	if e != nil {
		return e
	}
	positive2 := false
	if positive1 {
		positive2, e = n.sign(1, true)
		if e != nil {
			return e
		}
	}
	if positive1 && positive2 {
		return n.copy(6, 5)
	}
	negative1, e := n.sign(0, false)
	if e != nil {
		return e
	}
	positive2 = false
	if negative1 {
		positive2, e = n.sign(1, true)
		if e != nil {
			return e
		}
	}
	if negative1 && positive2 {
		v, e = q.NegWithInvocation(&n.values[5], &n.scalar)
		if e = n.store(7, v, e); e != nil {
			return e
		}
		v, e = q.AddWithInvocation(&n.values[7], &n.values[1], &n.scalar)
		if e = n.store(8, v, e); e != nil {
			return e
		}
		if e = n.unsignedNorm(8, 2, width); e != nil {
			return e
		}
	} else {
		positive1, e = n.sign(0, true)
		if e != nil {
			return e
		}
		negative2 := false
		if positive1 {
			negative2, e = n.sign(1, false)
			if e != nil {
				return e
			}
		}
		if positive1 && negative2 {
			v, e = q.AddWithInvocation(&n.values[5], &n.values[1], &n.scalar)
			if e = n.store(8, v, e); e != nil {
				return e
			}
			if e = n.unsignedNorm(8, 2, width); e != nil {
				return e
			}
		} else {
			v, e = q.NegWithInvocation(&n.values[5], &n.scalar)
			if e = n.store(7, v, e); e != nil {
				return e
			}
			if e = n.unsignedNorm(7, 2, width); e != nil {
				return e
			}
		}
	}
	if e = n.copy(6, 2); e != nil {
		return e
	}
	return n.End(2, 8, 7)
}
