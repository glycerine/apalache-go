package ast

// Source: src/ast/rewriter/poly_rewriter_def.h:428-451;
// src/ast/rewriter/bv_rewriter.h:36; src/util/rational.h:238-240,421-423.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *ASTManager) BitVectorPolynomialHoistLess(left, right ASTHandle) (bool, error) {
	return m.bitVectorPolynomialHoistLess(left, right, nil)
}

// BitVectorPolynomialHoistLessWithInvocation names the comparator's exact
// copied coefficient locals and comparison children. Operands remain borrowed.
func (m *ASTManager) BitVectorPolynomialHoistLessWithInvocation(left, right ASTHandle, c *ASTBitVectorNumeralInvocation) (bool, error) {
	if c == nil {
		return false, ErrASTHandle
	}
	return m.bitVectorPolynomialHoistLess(left, right, c)
}

func (m *ASTManager) bitVectorPolynomialCoefficientProduct(t astNodeRef, coefficient *unumeral.OwnedRational, c *unumeral.RationalServiceInvocation) (bool, astNodeRef, error) {
	mul, e := m.bitVectorAppKind(t, 6)
	if e != nil || !mul {
		return false, astNodeRef{}, e
	}
	app := m.appAt(t.slot)
	if app.count != 2 {
		return false, astNodeRef{}, nil
	}
	first, e := m.readInternal(app.args, 0)
	if e != nil {
		return false, astNodeRef{}, e
	}
	var width uint32
	yes, e := m.readBitVectorNumeralWithInvocation(first, coefficient, &width, c)
	if e != nil || !yes {
		return false, astNodeRef{}, e
	}
	pp, e := m.readInternal(app.args, 1)
	return true, pp, e
}

func (m *ASTManager) bitVectorPolynomialHoistLess(left, right ASTHandle, c *ASTBitVectorNumeralInvocation) (value bool, result error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	var receipt *astBVNormalizationReceipt
	if c != nil {
		if e := c.prepare(m); e != nil {
			return false, e
		}
		defer c.finish(&result)
		receipt = &c.receipt
	}
	l, e := m.requireExpr(left)
	if e != nil {
		return false, e
	}
	r, e := m.requireExpr(right)
	if e != nil {
		return false, e
	}
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	f, e := m.beginBVNumeralFrame()
	if e != nil {
		return false, e
	}
	f.polynomialHoistCompare = true
	if receipt != nil {
		*receipt = astBVNormalizationReceipt{frame: f, invocation: f.invocation}
	}
	defer f.finishNumeral(&result, receipt)
	for i := 0; i < 2; i++ {
		f.values[i], e = m.numerals.NewValue()
		if e != nil {
			return false, e
		}
		f.placed[i], f.complete[i] = true, true
	}
	mul1, pp1, e := m.bitVectorPolynomialCoefficientProduct(l, &f.values[0], &f.children[0])
	if e != nil {
		return false, e
	}
	mul2, pp2, e := m.bitVectorPolynomialCoefficientProduct(r, &f.values[1], &f.children[1])
	if e != nil {
		return false, e
	}
	if !mul1 && mul2 {
		return true, nil
	}
	if mul1 && !mul2 {
		return false, nil
	}
	if !mul1 && !mul2 {
		return m.nodeAt(l.slot).id < m.nodeAt(r.slot).id, nil
	}
	less, e := m.numerals.LessWithInvocation(&f.values[0], &f.values[1], &f.children[2])
	if e != nil {
		return false, e
	}
	if less {
		return true, nil
	}
	greater, e := m.numerals.GreaterWithInvocation(&f.values[0], &f.values[1], &f.children[3])
	if e != nil {
		return false, e
	}
	if greater {
		return false, nil
	}
	return m.nodeAt(pp1.slot).id < m.nodeAt(pp2.slot).id, nil
}
