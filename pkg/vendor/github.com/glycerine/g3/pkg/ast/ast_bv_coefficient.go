package ast

// Source: src/ast/rewriter/bv_rewriter.h:39; src/ast/bv_decl_plugin.cpp:753-765;
// src/util/rational.h:131-134.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	// NormalizeBitVectorCoefficient implements the BV polynomial configuration's
	// unsigned norm followed by copy assignment into the existing coefficient.
)

func (m *ASTManager) NormalizeBitVectorCoefficient(value *unumeral.OwnedRational, sort ASTHandle) error {
	return m.normalizeBitVectorCoefficient(value, sort, nil)
}

// NormalizeBitVectorCoefficientWithInvocation captures this exact operation
// so older caller locals can join its stopped children before source DT.
func (m *ASTManager) NormalizeBitVectorCoefficientWithInvocation(value *unumeral.OwnedRational, sort ASTHandle, c *ASTBitVectorNumeralInvocation) error {
	if c == nil {
		return ErrASTHandle
	}
	return m.normalizeBitVectorCoefficient(value, sort, c)
}

func (m *ASTManager) normalizeBitVectorCoefficient(value *unumeral.OwnedRational, sort ASTHandle, c *ASTBitVectorNumeralInvocation) (result error) {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	var receipt *astBVNormalizationReceipt
	if c != nil {
		if e := c.prepare(m); e != nil {
			return e
		}
		defer c.finish(&result)
		receipt = &c.receipt
	}
	r, e := m.requireKind(sort, ASTSort)
	if e != nil {
		return e
	}
	width, e := m.bitVectorSortSize(r, nil)
	if e != nil {
		return e
	}
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	f, e := m.beginBVNumeralFrame()
	if e != nil {
		return e
	}
	f.coefficientNormalization = true
	if receipt != nil {
		*receipt = astBVNormalizationReceipt{frame: f, invocation: f.invocation}
	}
	defer f.finishNumeral(&result, receipt)
	// Capture the actual norm child at entry. Its returned value remains a
	// distinct full-expression temporary until the copy assignment completes.
	f.values[0], e = m.normalizeBitVectorWithFrame(value, width, false, nil, &f.normalizationChild)
	if e != nil {
		return e
	}
	f.placed[0], f.complete[0] = true, true
	return m.numerals.CopyWithInvocation(value, &f.values[0], &f.children[0])
}
