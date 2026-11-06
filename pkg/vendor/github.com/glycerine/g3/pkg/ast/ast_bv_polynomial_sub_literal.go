package ast

// Source: src/ast/rewriter/poly_rewriter_def.h:689-691;
// src/util/rational.h:46; src/util/obj_ref.h:39-43; src/ast/ast.h:2259.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorPolynomialSubtractionLiteral owns the -1 full expression while
// the caller's expr_ref construction acquires its resulting numeral.
type BitVectorPolynomialSubtractionLiteral struct {
	sourceCause                     error
	self                            *BitVectorPolynomialSubtractionLiteral
	manager                         *ASTManager
	value                           unumeral.OwnedRational
	placed                          bool
	scalar                          unumeral.RationalServiceInvocation
	call                            ASTBitVectorNumeralInvocation
	header                          uint64
	started, ended, stopped, closed bool
}

func NewBitVectorPolynomialSubtractionLiteral(m *ASTManager) (*BitVectorPolynomialSubtractionLiteral, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorPolynomialSubtractionLiteral{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorPolynomialSubtractionLiteral{manager: m, header: bytes}
	n.self = n
	return n, nil
}
func (n *BitVectorPolynomialSubtractionLiteral) constant(sort ASTHandle) (node ASTHandle, result error) {
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
	r, e := n.manager.mkBitVectorNumeralCaptured(&n.value, width, &n.call.receipt)
	return n.manager.external(r), e
}
func (n *BitVectorPolynomialSubtractionLiteral) ConstructReference(sort ASTHandle, out *ASTRef) (err error) {
	if n == nil || n.self != n || n.closed || out == nil {
		return ErrASTHandle
	}
	if n.started || n.stopped {
		return ErrASTPending
	}
	if *out != (ASTRef{}) {
		return ErrASTHandle
	}
	n.started = true
	defer func() {
		if err != nil {
			n.stopped = true
		}
	}()
	v, e := n.manager.numerals.NewValue()
	if e != nil {
		return e
	}
	n.value = v
	n.placed = true
	if e = n.manager.numerals.SetInt64WithInvocation(&n.value, -1, &n.scalar); e != nil {
		return e
	}
	if e = n.scalar.SourceReady(); e != nil {
		return e
	}
	if e = n.scalar.Cleanup(false); e != nil {
		return e
	}
	if e = n.scalar.Cleanup(true); e != nil {
		return e
	}
	if e = n.scalar.EndContext(); e != nil {
		return e
	}
	n.scalar = unumeral.RationalServiceInvocation{}
	node, e := n.constant(sort)
	if e != nil {
		return e
	}
	ref, e := n.manager.NewRef(node)
	if e != nil {
		return e
	}
	*out = ref // expr_ref constructor INC precedes the outer temporary DT
	if e = n.SourceReady(); e != nil {
		return e
	}
	if e = n.manager.numerals.Del(&n.value); e != nil {
		n.sourceCause = e
		return e
	}
	n.placed = false
	n.ended = true
	return nil
}
func (n *BitVectorPolynomialSubtractionLiteral) SourceReady() error {
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
func (n *BitVectorPolynomialSubtractionLiteral) Close() error {
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
	if n.placed {
		if e := n.manager.numerals.CleanupValue(&n.value); e != nil {
			return e
		}
		n.placed = false
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

// Source: src/ast/rewriter/poly_rewriter_def.h:70-74.
func (m *ASTManager) IsBitVectorPolynomialZeroWithInvocation(arg ASTHandle, c *ASTBitVectorNumeralInvocation) (zero bool, result error) {
	if c == nil {
		return false, ErrASTHandle
	}
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	if e := c.prepare(m); e != nil {
		return false, e
	}
	defer c.finish(&result)
	r, e := m.requireExpr(arg)
	if e != nil {
		return false, e
	}
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	return m.bitVectorPolynomialZero(r, &c.receipt)
}
