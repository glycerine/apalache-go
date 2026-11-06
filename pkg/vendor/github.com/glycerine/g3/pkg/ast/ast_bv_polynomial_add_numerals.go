package ast

// Source: src/ast/rewriter/poly_rewriter_def.h:530-534,563,570-587,606-609;
// src/ast/rewriter/poly_rewriter_def.h:708-883;
// src/util/rational.h:131-134,151-153; src/util/buffer.h:29-75,146-151.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorPolynomialAdditionNumerals owns the addition/cancellation caller's c and a,
// coefficient buffer and exact numeric children. The manager is borrowed.
type BitVectorPolynomialAdditionNumerals struct {
	self                            *BitVectorPolynomialAdditionNumerals
	manager                         *ASTManager
	values                          [2]unumeral.OwnedRational
	placed                          [2]bool
	scalar                          unumeral.RationalServiceInvocation
	call                            ASTBitVectorNumeralInvocation
	coeffs                          *unumeral.RationalBuffer16
	header                          uint64
	started, stopped, ended, closed bool
	coefficientsEnded               bool
}

func NewBitVectorPolynomialAdditionNumerals(m *ASTManager) (*BitVectorPolynomialAdditionNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorPolynomialAdditionNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorPolynomialAdditionNumerals{manager: m, header: bytes}
	n.self = n
	return n, nil
}
func (n *BitVectorPolynomialAdditionNumerals) check() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorPolynomialAdditionNumerals) open() error {
	if e := n.check(); e != nil {
		return e
	}
	if !n.started {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorPolynomialAdditionNumerals) Begin() error {
	if e := n.check(); e != nil {
		return e
	}
	if n.started {
		return ErrASTPending
	}
	n.started = true
	for i := range n.values {
		v, e := n.manager.numerals.NewValue()
		if e != nil {
			n.stopped = true
			return e
		}
		n.values[i] = v
		n.placed[i] = true
	}
	return nil
}
func (n *BitVectorPolynomialAdditionNumerals) retire() error {
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
func (n *BitVectorPolynomialAdditionNumerals) Read(arg ASTHandle) (bool, error) {
	if e := n.open(); e != nil {
		return false, e
	}
	var width uint32
	yes, e := n.manager.BitVectorNumeralValueWithInvocation(arg, &n.values[1], &width, &n.scalar)
	if e != nil {
		n.stopped = true
		return false, e
	}
	if e = n.retire(); e != nil {
		n.stopped = true
	}
	return yes, e
}
func (n *BitVectorPolynomialAdditionNumerals) AddConstant() error {
	if e := n.open(); e != nil {
		return e
	}
	e := n.manager.numerals.AddAssignWithInvocation(&n.values[0], &n.values[1], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorPolynomialAdditionNumerals) Normalize(sort ASTHandle, coefficient bool) error {
	if e := n.open(); e != nil {
		return e
	}
	i := 0
	if coefficient {
		i = 1
	}
	e := n.manager.NormalizeBitVectorCoefficientWithInvocation(&n.values[i], sort, &n.call)
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorPolynomialAdditionNumerals) Zero(coefficient bool) (bool, error) {
	if e := n.open(); e != nil {
		return false, e
	}
	i := 0
	if coefficient {
		i = 1
	}
	return n.manager.numerals.IsZero(&n.values[i])
}
func (n *BitVectorPolynomialAdditionNumerals) PowerProduct(arg ASTHandle) (ASTHandle, error) {
	if e := n.open(); e != nil {
		return ASTHandle{}, e
	}
	pp, e := n.manager.ReadBitVectorPolynomialPowerProductWithInvocation(arg, &n.values[1], &n.call)
	if e != nil {
		n.stopped = true
	}
	return pp, e
}
func (n *BitVectorPolynomialAdditionNumerals) BeginCoefficients() error {
	if e := n.open(); e != nil {
		return e
	}
	if n.coeffs != nil {
		return ErrASTPending
	}
	var e error
	n.coeffs, e = unumeral.NewRationalBuffer16(n.manager.numerals)
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorPolynomialAdditionNumerals) CoefficientCount() (uint32, error) {
	if e := n.open(); e != nil {
		return 0, e
	}
	if n.coeffs == nil {
		return 0, ErrASTPending
	}
	return n.coeffs.Len()
}
func (n *BitVectorPolynomialAdditionNumerals) AppendCoefficient() error {
	if e := n.open(); e != nil {
		return e
	}
	if n.coeffs == nil {
		return ErrASTPending
	}
	e := n.coeffs.PushCopy(&n.values[1])
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorPolynomialAdditionNumerals) AccumulateCoefficient(pos uint32) error {
	if e := n.open(); e != nil {
		return e
	}
	if n.coeffs == nil {
		return ErrASTPending
	}
	dst, e := n.coeffs.At(pos)
	if e != nil {
		return e
	}
	e = n.manager.numerals.AddAssignWithInvocation(dst, &n.values[1], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorPolynomialAdditionNumerals) LoadCoefficient(pos uint32) error {
	if e := n.open(); e != nil {
		return e
	}
	if n.coeffs == nil {
		return ErrASTPending
	}
	src, e := n.coeffs.At(pos)
	if e != nil {
		return e
	}
	e = n.manager.numerals.CopyWithInvocation(&n.values[1], src, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorPolynomialAdditionNumerals) Constant(sort ASTHandle) (ASTHandle, error) {
	return n.constant(sort, 0)
}

// Source: src/ast/rewriter/poly_rewriter_def.h:842.
func (n *BitVectorPolynomialAdditionNumerals) CoefficientConstant(sort ASTHandle) (ASTHandle, error) {
	return n.constant(sort, 1)
}
func (n *BitVectorPolynomialAdditionNumerals) constant(sort ASTHandle, index int) (node ASTHandle, result error) {
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
	r, e := n.manager.mkBitVectorNumeralCaptured(&n.values[index], width, &n.call.receipt)
	return n.manager.external(r), e
}
func (n *BitVectorPolynomialAdditionNumerals) Product(sort, pp ASTHandle) (ASTHandle, error) {
	if e := n.open(); e != nil {
		return ASTHandle{}, e
	}
	node, e := n.manager.MkBitVectorPolynomialCoefficientWithInvocation(&n.values[1], sort, pp, &n.call)
	if e != nil {
		n.stopped = true
	}
	return node, e
}
func (n *BitVectorPolynomialAdditionNumerals) EndCoefficients() error {
	if e := n.open(); e != nil {
		return e
	}
	if n.coeffs == nil {
		return nil
	}
	if e := n.coeffs.SourceEnd(); e != nil {
		n.stopped = true
		return e
	}
	if e := n.coeffs.Close(); e != nil {
		return e
	}
	n.coefficientsEnded = true
	return nil
}
func (n *BitVectorPolynomialAdditionNumerals) SourceChildren() error {
	if n == nil || n.self != n {
		return ErrASTHandle
	}
	if e := n.scalar.SourceReady(); e != nil {
		return e
	}
	if n.coeffs != nil {
		_, source, _ := n.coeffs.Causes()
		if source != nil {
			return source
		}
	}
	return n.manager.BitVectorNumeralInvocationSourceReady(&n.call)
}
func (n *BitVectorPolynomialAdditionNumerals) SourceEnd() error {
	if e := n.open(); e != nil {
		return e
	}
	if n.coeffs != nil && !n.coefficientsEnded {
		return ErrASTPending
	}
	if e := n.SourceChildren(); e != nil {
		n.stopped = true
		return e
	}
	for i := 1; i >= 0; i-- {
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
func (n *BitVectorPolynomialAdditionNumerals) Close() error {
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
	if n.coeffs != nil {
		if e := n.coeffs.Close(); e != nil {
			return e
		}
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

// Source: src/ast/rewriter/poly_rewriter_def.h:720-721.
func (n *BitVectorPolynomialAdditionNumerals) BeginCancellation() error {
	if e := n.check(); e != nil {
		return e
	}
	if n.started {
		return ErrASTPending
	}
	n.started = true
	v, e := n.manager.numerals.NewValue()
	if e != nil {
		n.stopped = true
		return e
	}
	n.values[0] = v
	n.placed[0] = true
	e = n.manager.numerals.SetInt64WithInvocation(&n.values[0], 0, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		n.stopped = true
		return e
	}
	v, e = n.manager.numerals.NewValue()
	if e != nil {
		n.stopped = true
		return e
	}
	n.values[1] = v
	n.placed[1] = true
	return nil
}

// Source: src/ast/rewriter/poly_rewriter_def.h:743.
func (n *BitVectorPolynomialAdditionNumerals) SubtractConstant() error {
	if e := n.open(); e != nil {
		return e
	}
	e := n.manager.numerals.SubAssignWithInvocation(&n.values[0], &n.values[1], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		n.stopped = true
	}
	return e
}

// Source: src/ast/rewriter/poly_rewriter_def.h:799.
func (n *BitVectorPolynomialAdditionNumerals) SubtractCoefficient(pos uint32) error {
	if e := n.open(); e != nil {
		return e
	}
	if n.coeffs == nil {
		return ErrASTPending
	}
	dst, e := n.coeffs.At(pos)
	if e != nil {
		return e
	}
	e = n.manager.numerals.SubAssignWithInvocation(dst, &n.values[1], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		n.stopped = true
	}
	return e
}

// Source: src/ast/rewriter/poly_rewriter_def.h:840,869.
func (n *BitVectorPolynomialAdditionNumerals) Negate(coefficient bool) error {
	if e := n.open(); e != nil {
		return e
	}
	i := 0
	if coefficient {
		i = 1
	}
	e := n.manager.numerals.NegAssignWithInvocation(&n.values[i], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorPolynomialAdditionNumerals) CoefficientIsMinusOne() (bool, error) {
	if e := n.open(); e != nil {
		return false, e
	}
	return n.manager.numerals.IsMinusOne(&n.values[1])
}
func (n *BitVectorPolynomialAdditionNumerals) CoefficientIsOne() (bool, error) {
	if e := n.open(); e != nil {
		return false, e
	}
	return n.manager.numerals.IsOne(&n.values[1])
}
