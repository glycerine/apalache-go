package ast

// Source: src/ast/rewriter/poly_rewriter_def.h:410-416;
// src/ast/rewriter/bv_rewriter.h:36; src/util/rational.h:46,131-134.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *ASTManager) ReadBitVectorPolynomialPowerProduct(arg ASTHandle, coefficient *unumeral.OwnedRational) (ASTHandle, error) {
	return m.readBitVectorPolynomialPowerProduct(arg, coefficient, nil)
}

// ReadBitVectorPolynomialPowerProductWithInvocation names the original
// stored-coefficient copy or fallback numeral-one full expression. The result
// is borrowed; the existing caller coefficient is the physical receiver.
func (m *ASTManager) ReadBitVectorPolynomialPowerProductWithInvocation(arg ASTHandle, coefficient *unumeral.OwnedRational, c *ASTBitVectorNumeralInvocation) (ASTHandle, error) {
	if c == nil {
		return ASTHandle{}, ErrASTHandle
	}
	return m.readBitVectorPolynomialPowerProduct(arg, coefficient, c)
}

func (m *ASTManager) readBitVectorPolynomialPowerProduct(arg ASTHandle, coefficient *unumeral.OwnedRational, c *ASTBitVectorNumeralInvocation) (node ASTHandle, result error) {
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
	r, e := m.requireExpr(arg)
	if e != nil {
		return node, e
	}
	mul, e := m.bitVectorAppKind(r, 6)
	if e != nil {
		return node, e
	}
	var first astNodeRef
	binary := mul && m.appAt(r.slot).count == 2
	if binary {
		first, e = m.readInternal(m.appAt(r.slot).args, 0)
		if e != nil {
			return node, e
		}
	}
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	f, e := m.beginBVNumeralFrame()
	if e != nil {
		return node, e
	}
	f.polynomialPowerProduct = true
	if receipt != nil {
		*receipt = astBVNormalizationReceipt{frame: f, invocation: f.invocation}
	}
	defer f.finishNumeral(&result, receipt)
	if binary {
		var width uint32
		matched, e := m.readBitVectorNumeralWithInvocation(first, coefficient, &width, &f.children[2])
		if e != nil {
			return node, e
		}
		if matched {
			pp, e := m.readInternal(m.appAt(r.slot).args, 1)
			return m.external(pp), e
		}
	}
	// a = numeral(1) constructs a distinct temporary and copy-assigns it;
	// assigning a scalar directly or swapping the receiver would change this.
	f.values[0], e = m.numerals.NewValue()
	if e != nil {
		return node, e
	}
	f.placed[0] = true
	if e = m.numerals.SetInt64WithInvocation(&f.values[0], 1, &f.children[0]); e != nil {
		return node, e
	}
	f.complete[0] = true
	if e = m.numerals.CopyWithInvocation(coefficient, &f.values[0], &f.children[1]); e != nil {
		return node, e
	}
	return arg, nil
}
