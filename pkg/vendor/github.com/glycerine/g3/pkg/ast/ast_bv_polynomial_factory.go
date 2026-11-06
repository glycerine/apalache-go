package ast

// Source: src/ast/rewriter/poly_rewriter_def.h:47-53,70-74,78-124,127-138;
// src/ast/rewriter/bv_rewriter.h:35-43; src/util/rational.h:43,46,131-134.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *ASTManager) MkBitVectorPolynomialAdd(sort ASTHandle, args AppArguments) (ASTHandle, error) {
	return m.bitVectorPolynomialApp(sort, args, 4, 0, nil)
}
func (m *ASTManager) MkBitVectorPolynomialMul(sort ASTHandle, args AppArguments) (ASTHandle, error) {
	return m.bitVectorPolynomialApp(sort, args, 6, 1, nil)
}
func (m *ASTManager) MkBitVectorPolynomialAddWithInvocation(sort ASTHandle, args AppArguments, c *ASTBitVectorNumeralInvocation) (ASTHandle, error) {
	if c == nil {
		return ASTHandle{}, ErrASTHandle
	}
	return m.bitVectorPolynomialApp(sort, args, 4, 0, c)
}
func (m *ASTManager) MkBitVectorPolynomialMulWithInvocation(sort ASTHandle, args AppArguments, c *ASTBitVectorNumeralInvocation) (ASTHandle, error) {
	if c == nil {
		return ASTHandle{}, ErrASTHandle
	}
	return m.bitVectorPolynomialApp(sort, args, 6, 1, c)
}

// The original BV Config disables use_power. Only an empty application reads
// the current sort; singleton results borrow their input without a factory.
func (m *ASTManager) bitVectorPolynomialApp(sort ASTHandle, args AppArguments, kind int32, unit int64, c *ASTBitVectorNumeralInvocation) (node ASTHandle, result error) {
	if args.Count() == 1 && c == nil {
		return args.At(0)
	}
	if e := m.lock(); e != nil {
		return node, e
	}
	defer m.mu.Unlock()
	var receipt *astBVNormalizationReceipt
	if c != nil {
		if e := c.prepare(m); e != nil {
			return node, e
		}
		defer c.finish(&result)
		receipt = &c.receipt
	}
	switch args.Count() {
	case 0:
		previous := m.formatExecuting
		m.formatExecuting = true
		defer func() { m.formatExecuting = previous }()
		r, e := m.bitVectorPolynomialIdentity(sort, unit, receipt, nil, false)
		return m.external(r), e
	case 1:
		return args.At(0)
	default:
		return m.bitVectorArgumentsCore(kind, args)
	}
}

func (m *ASTManager) bitVectorPolynomialIdentity(sort ASTHandle, unit int64, receipt *astBVNormalizationReceipt, publish *ASTHandle, assign bool) (node astNodeRef, result error) {
	f, e := m.beginBVNumeralFrame()
	if e != nil {
		return node, e
	}
	f.polynomialLiteral = true
	if receipt != nil {
		*receipt = astBVNormalizationReceipt{frame: f, invocation: f.invocation}
	}
	defer f.finishNumeral(&result, receipt)
	// numeral(int) is constructed before entering the sort-based factory.
	f.values[5], e = m.numerals.NewValue()
	if e != nil {
		return node, e
	}
	f.placed[5] = true
	if e = m.numerals.SetInt64WithInvocation(&f.values[5], unit, &f.children[9]); e != nil {
		return node, e
	}
	f.complete[5] = true
	s, e := m.requireKind(sort, ASTSort)
	if e != nil {
		return node, e
	}
	matched, e := m.bitVectorSortRecognized(s)
	if e != nil || !matched {
		return node, e
	}
	width, e := m.bitVectorSortSize(s, nil)
	if e != nil {
		return node, e
	}
	if e = f.copyConstruct(0, &f.values[5]); e != nil {
		return node, e
	}
	node, result = m.bitVectorNumeralAppCore(f, width)
	if result != nil || !assign {
		return node, result
	}
	// Finish the numeral factory's locals before returning its pointer to the
	// caller assignment. The outer integer literal stays live through the store.
	if result = f.joinNormalizationChild(false, false); result != nil {
		return node, result
	}
	for _, i := range f.childOrder() {
		if i >= 0 {
			if result = f.children[i].SourceReady(); result != nil {
				f.sourceCause = result
				return node, result
			}
		}
	}
	for _, i := range f.valueOrder() {
		if i != 5 {
			if result = f.destroyOne(i, false); result != nil {
				return node, result
			}
		}
	}
	if publish == nil {
		return node, ErrASTHandle
	}
	*publish = m.external(node)
	return node, nil
}

// AssignBitVectorPolynomialZeroWithInvocation preserves the caller's pointer
// assignment before the rational(0) full-expression temporary ends.
func (m *ASTManager) AssignBitVectorPolynomialZeroWithInvocation(sort ASTHandle, out *ASTHandle, c *ASTBitVectorNumeralInvocation) (result error) {
	if c == nil {
		return ErrASTHandle
	}
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if e := c.prepare(m); e != nil {
		return e
	}
	defer c.finish(&result)
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	_, result = m.bitVectorPolynomialIdentity(sort, 0, &c.receipt, out, true)
	return result
}

func (m *ASTManager) MkBitVectorPolynomialCoefficient(value *unumeral.OwnedRational, sort, arg ASTHandle) (ASTHandle, error) {
	return m.bitVectorPolynomialCoefficient(value, sort, arg, nil)
}
func (m *ASTManager) MkBitVectorPolynomialCoefficientWithInvocation(value *unumeral.OwnedRational, sort, arg ASTHandle, c *ASTBitVectorNumeralInvocation) (ASTHandle, error) {
	if c == nil {
		return ASTHandle{}, ErrASTHandle
	}
	return m.bitVectorPolynomialCoefficient(value, sort, arg, c)
}

func (m *ASTManager) bitVectorPolynomialCoefficient(value *unumeral.OwnedRational, sort, arg ASTHandle, c *ASTBitVectorNumeralInvocation) (node ASTHandle, result error) {
	if e := m.lock(); e != nil {
		return node, e
	}
	defer m.mu.Unlock()
	var receipt *astBVNormalizationReceipt
	if c != nil {
		if e := c.prepare(m); e != nil {
			return node, e
		}
		defer c.finish(&result)
		receipt = &c.receipt
	}
	one, e := m.numerals.IsOne(value)
	if e != nil {
		return node, e
	}
	if one {
		return arg, nil
	}
	r, e := m.requireExpr(arg)
	if e != nil {
		return node, e
	}
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	zero, e := m.bitVectorPolynomialZero(r, receipt)
	if e != nil {
		return node, e
	}
	if zero {
		return arg, nil
	}
	s, e := m.requireKind(sort, ASTSort)
	if e != nil {
		return node, e
	}
	matched, e := m.bitVectorSortRecognized(s)
	if e != nil {
		return node, e
	}
	var literal astNodeRef
	if matched {
		width, e := m.bitVectorSortSize(s, nil)
		if e != nil {
			return node, e
		}
		literal, e = m.mkBitVectorNumeralCaptured(value, width, receipt)
		if e != nil {
			return node, e
		}
	}
	args := [2]ASTHandle{m.external(literal), arg}
	m.formatExecuting = previous
	return m.bitVectorArgumentsCore(6, SliceArguments(args[:]))
}

// poly_rewriter::is_zero owns a copied stored numeral. A symbolic input erases
// the source allocation-free default rational, and the completed read's local
// ends before a subsequent coefficient numeral factory begins.
func (m *ASTManager) bitVectorPolynomialZero(arg astNodeRef, receipt *astBVNormalizationReceipt) (zero bool, result error) {
	matched, e := m.bitVectorAppKind(arg, 0)
	if e != nil || !matched {
		return false, e
	}
	f, e := m.beginBVNumeralFrame()
	if e != nil {
		return false, e
	}
	f.polynomialZero = true
	if receipt != nil {
		*receipt = astBVNormalizationReceipt{frame: f, invocation: f.invocation}
	}
	defer f.finishNumeral(&result, receipt)
	f.values[0], e = m.numerals.NewValue()
	if e != nil {
		return false, e
	}
	f.placed[0], f.complete[0] = true, true
	var width uint32
	_, e = m.readBitVectorNumeralWithInvocation(arg, &f.values[0], &width, &f.children[2])
	if e != nil {
		return false, e
	}
	return m.numerals.IsZero(&f.values[0])
}
