package ast

// Source: src/ast/rewriter/poly_rewriter_def.h:667-681;
// src/util/rational.h:43,46,242-244.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorPolynomialNegationNumerals owns the default a and reached -1 full
// expression temporary, with exact scalar and AST numeral children.
type BitVectorPolynomialNegationNumerals struct {
	self                   *BitVectorPolynomialNegationNumerals
	manager                *ASTManager
	values                 [2]unumeral.OwnedRational
	placed                 [2]bool
	scalar                 unumeral.RationalServiceInvocation
	call                   ASTBitVectorNumeralInvocation
	header                 uint64
	sourceCause            error
	stopped, ended, closed bool
}

func NewBitVectorPolynomialNegationNumerals(m *ASTManager) (*BitVectorPolynomialNegationNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorPolynomialNegationNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorPolynomialNegationNumerals{manager: m, header: bytes}
	n.self = n
	return n, nil
}
func (n *BitVectorPolynomialNegationNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorPolynomialNegationNumerals) Begin() error {
	if e := n.open(); e != nil {
		return e
	}
	if n.placed[0] {
		return ErrASTPending
	}
	v, e := n.manager.numerals.NewValue()
	if e != nil {
		n.stopped = true
		return e
	}
	n.values[0] = v
	n.placed[0] = true
	return nil
}
func (n *BitVectorPolynomialNegationNumerals) retire() error {
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
func (n *BitVectorPolynomialNegationNumerals) Read(arg ASTHandle) (bool, error) {
	if e := n.open(); e != nil {
		return false, e
	}
	if !n.placed[0] {
		return false, ErrASTPending
	}
	var width uint32
	yes, e := n.manager.BitVectorNumeralValueWithInvocation(arg, &n.values[0], &width, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		n.stopped = true
	}
	return yes, e
}
func (n *BitVectorPolynomialNegationNumerals) Negate() error {
	if e := n.open(); e != nil {
		return e
	}
	if !n.placed[0] {
		return ErrASTPending
	}
	e := n.manager.numerals.NegAssignWithInvocation(&n.values[0], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorPolynomialNegationNumerals) Normalize(sort ASTHandle) error {
	if e := n.open(); e != nil {
		return e
	}
	if !n.placed[0] {
		return ErrASTPending
	}
	e := n.manager.NormalizeBitVectorCoefficientWithInvocation(&n.values[0], sort, &n.call)
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorPolynomialNegationNumerals) Constant(sort ASTHandle) (node ASTHandle, result error) {
	if e := n.open(); e != nil {
		return node, e
	}
	if !n.placed[0] {
		return node, ErrASTPending
	}
	defer func() {
		if result != nil {
			n.stopped = true
		}
	}()
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
	r, e := n.manager.mkBitVectorNumeralCaptured(&n.values[0], width, &n.call.receipt)
	return n.manager.external(r), e
}
func (n *BitVectorPolynomialNegationNumerals) Product(sort, arg ASTHandle) (ASTHandle, error) {
	if e := n.open(); e != nil {
		return ASTHandle{}, e
	}
	if !n.placed[0] || n.placed[1] {
		return ASTHandle{}, ErrASTPending
	}
	v, e := n.manager.numerals.NewValue()
	if e != nil {
		n.stopped = true
		return ASTHandle{}, e
	}
	n.values[1] = v
	n.placed[1] = true
	e = n.manager.numerals.SetInt64WithInvocation(&n.values[1], -1, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		n.stopped = true
		return ASTHandle{}, e
	}
	node, e := n.manager.MkBitVectorPolynomialCoefficientWithInvocation(&n.values[1], sort, arg, &n.call)
	if e != nil {
		n.stopped = true
	}
	return node, e
}
func (n *BitVectorPolynomialNegationNumerals) SourceChildren() error {
	if n == nil || n.self != n {
		return ErrASTHandle
	}
	if n.sourceCause != nil {
		return n.sourceCause
	}
	if e := n.scalar.SourceReady(); e != nil {
		return e
	}
	return n.manager.BitVectorNumeralInvocationSourceReady(&n.call)
}
func (n *BitVectorPolynomialNegationNumerals) SourceEnd() error {
	if e := n.open(); e != nil {
		return e
	}
	if e := n.SourceChildren(); e != nil {
		n.stopped = true
		n.sourceCause = e
		return e
	}
	for i := 1; i >= 0; i-- {
		if n.placed[i] {
			if e := n.manager.numerals.Del(&n.values[i]); e != nil {
				n.stopped = true
				n.sourceCause = e
				return e
			}
			n.placed[i] = false
		}
	}
	n.ended = true
	return nil
}
func (n *BitVectorPolynomialNegationNumerals) Close() error {
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
	for i := 1; i >= 0; i-- {
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
