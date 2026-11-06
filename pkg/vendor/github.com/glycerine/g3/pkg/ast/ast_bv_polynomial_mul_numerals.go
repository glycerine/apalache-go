package ast

// Source: src/ast/rewriter/poly_rewriter_def.h:202-218,236-249,259-263;
// src/util/rational.h:43,46,161-164.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorPolynomialMultiplicationNumerals owns a, c, and the reached
// c_prime locals, with exact reads, compound multiplication and normalization.
type BitVectorPolynomialMultiplicationNumerals struct {
	self                   *BitVectorPolynomialMultiplicationNumerals
	manager                *ASTManager
	values                 [4]unumeral.OwnedRational
	placed                 [4]bool
	scalar                 unumeral.RationalServiceInvocation
	call                   ASTBitVectorNumeralInvocation
	header                 uint64
	stopped, ended, closed bool
}

func NewBitVectorPolynomialMultiplicationNumerals(m *ASTManager) (*BitVectorPolynomialMultiplicationNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorPolynomialMultiplicationNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorPolynomialMultiplicationNumerals{manager: m, header: bytes}
	n.self = n
	return n, nil
}
func (n *BitVectorPolynomialMultiplicationNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorPolynomialMultiplicationNumerals) Place(i int) error {
	if e := n.open(); e != nil {
		return e
	}
	if i < 0 || i >= len(n.values) || n.placed[i] {
		return ErrASTDomain
	}
	v, e := n.manager.numerals.NewValue()
	if e != nil {
		n.stopped = true
		return e
	}
	n.values[i] = v
	n.placed[i] = true
	return nil
}
func (n *BitVectorPolynomialMultiplicationNumerals) retire() error {
	c := &n.scalar
	if e := c.SourceReady(); e != nil {
		return e
	}
	if e := c.Cleanup(false); e != nil {
		return e
	}
	if e := c.Cleanup(true); e != nil {
		return e
	}
	if e := c.EndContext(); e != nil {
		return e
	}
	*c = unumeral.RationalServiceInvocation{}
	return nil
}
func (n *BitVectorPolynomialMultiplicationNumerals) Read(arg ASTHandle, i int) (bool, error) {
	if e := n.open(); e != nil {
		return false, e
	}
	if i < 0 || i >= len(n.values) || !n.placed[i] {
		return false, ErrASTDomain
	}
	var width uint32
	yes, e := n.manager.BitVectorNumeralValueWithInvocation(arg, &n.values[i], &width, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		n.stopped = true
	}
	return yes, e
}
func (n *BitVectorPolynomialMultiplicationNumerals) SetOne() error {
	if e := n.Place(1); e != nil {
		return e
	}
	e := n.manager.numerals.SetInt64WithInvocation(&n.values[1], 1, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorPolynomialMultiplicationNumerals) One(i int) (bool, error) {
	if e := n.open(); e != nil {
		return false, e
	}
	if i < 0 || i >= len(n.values) || !n.placed[i] {
		return false, ErrASTDomain
	}
	return n.manager.numerals.IsOne(&n.values[i])
}
func (n *BitVectorPolynomialMultiplicationNumerals) Zero(i int) (bool, error) {
	if e := n.open(); e != nil {
		return false, e
	}
	if i < 0 || i >= len(n.values) || !n.placed[i] {
		return false, ErrASTDomain
	}
	return n.manager.numerals.IsZero(&n.values[i])
}
func (n *BitVectorPolynomialMultiplicationNumerals) Multiply(i int) error {
	if e := n.open(); e != nil {
		return e
	}
	if i < 0 || i >= len(n.values) || !n.placed[i] || !n.placed[1] {
		return ErrASTDomain
	}
	e := n.manager.numerals.MulAssignWithInvocation(&n.values[1], &n.values[i], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorPolynomialMultiplicationNumerals) Normalize(sort ASTHandle) error {
	if e := n.open(); e != nil {
		return e
	}
	if !n.placed[1] {
		return ErrASTDomain
	}
	e := n.manager.NormalizeBitVectorCoefficientWithInvocation(&n.values[1], sort, &n.call)
	if e != nil {
		n.stopped = true
	}
	return e
}

// DebugAlreadySimplified constructs the separate shadow c_prime in DEBUG_CODE.
func (n *BitVectorPolynomialMultiplicationNumerals) DebugAlreadySimplified(first ASTHandle) error {
	if e := n.Place(3); e != nil {
		return e
	}
	yes, e := n.Read(first, 3)
	if e != nil {
		return e
	}
	if !yes {
		return ErrASTDomain
	}
	equal, e := n.manager.numerals.Equal(&n.values[1], &n.values[3])
	if e != nil {
		return e
	}
	if !equal {
		return ErrASTDomain
	}
	if e = n.manager.numerals.Del(&n.values[3]); e != nil {
		n.stopped = true
		return e
	}
	n.placed[3] = false
	return nil
}
func (n *BitVectorPolynomialMultiplicationNumerals) Constant(sort ASTHandle) (node ASTHandle, result error) {
	if e := n.open(); e != nil {
		return node, e
	}
	if e := n.manager.lock(); e != nil {
		return node, e
	}
	defer n.manager.mu.Unlock()
	if e := n.call.prepare(n.manager); e != nil {
		return node, e
	}
	defer n.call.finish(&result)
	s, e := n.manager.requireKind(sort, ASTSort)
	if e != nil {
		return node, e
	}
	yes, e := n.manager.bitVectorSortRecognized(s)
	if e != nil || !yes {
		return node, e
	}
	width, e := n.manager.bitVectorSortSize(s, nil)
	if e != nil {
		return node, e
	}
	previous := n.manager.formatExecuting
	n.manager.formatExecuting = true
	defer func() { n.manager.formatExecuting = previous }()
	r, e := n.manager.mkBitVectorNumeralCaptured(&n.values[1], width, &n.call.receipt)
	return n.manager.external(r), e
}
func (n *BitVectorPolynomialMultiplicationNumerals) Product(sort, pp ASTHandle) (ASTHandle, error) {
	if e := n.open(); e != nil {
		return ASTHandle{}, e
	}
	node, e := n.manager.MkBitVectorPolynomialCoefficientWithInvocation(&n.values[1], sort, pp, &n.call)
	if e != nil {
		n.stopped = true
	}
	return node, e
}

func (n *BitVectorPolynomialMultiplicationNumerals) SourceChildren() error {
	if n == nil || n.self != n {
		return ErrASTHandle
	}
	if e := n.scalar.SourceReady(); e != nil {
		return e
	}
	return n.manager.BitVectorNumeralInvocationSourceReady(&n.call)
}
func (n *BitVectorPolynomialMultiplicationNumerals) SourceEnd() error {
	if e := n.open(); e != nil {
		return e
	}
	if e := n.SourceChildren(); e != nil {
		n.stopped = true
		return e
	}
	for i := 3; i >= 0; i-- {
		if n.placed[i] {
			if e := n.manager.numerals.Del(&n.values[i]); e != nil {
				n.stopped = true
				return e
			}
			n.placed[i] = false
		}
	}
	n.ended = true
	return nil
}
func (n *BitVectorPolynomialMultiplicationNumerals) Close() error {
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
