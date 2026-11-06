package ast

// Source: src/ast/bv_decl_plugin.cpp:767-773.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (m *ASTManager) BitVectorHasSignBit(n *unumeral.OwnedRational, bits uint32) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	return m.bitVectorHasSignBit(n, bits, nil)
}

// BitVectorHasSignBitWithInvocation captures the actual sign-test frame at entry.
func (m *ASTManager) BitVectorHasSignBitWithInvocation(n *unumeral.OwnedRational, bits uint32, c *ASTBitVectorNumeralInvocation) (sign bool, result error) {
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
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	return m.bitVectorHasSignBitCaptured(n, bits, nil, &c.receipt)
}

func (m *ASTManager) bitVectorHasSignBit(n *unumeral.OwnedRational, bits uint32, g *uparams.GlobalParamsService) (bool, error) {
	return m.bitVectorHasSignBitCaptured(n, bits, g, nil)
}
func (m *ASTManager) bitVectorHasSignBitCaptured(n *unumeral.OwnedRational, bits uint32, g *uparams.GlobalParamsService, captured *astBVNormalizationReceipt) (sign bool, result error) {
	enabled, e := astBVAssertionEnabled(g)
	if e != nil {
		return false, e
	}
	if enabled && bits == 0 {
		if e = uparams.NamespaceAssertionFailureOptional(g, "src/ast/bv_decl_plugin.cpp", 768, "bv_size > 0"); e != nil {
			return false, e
		}
	}
	f, e := m.beginBVNumeralFrame()
	if e != nil {
		return false, e
	}
	if captured != nil {
		*captured = astBVNormalizationReceipt{frame: f, invocation: f.invocation}
	}
	defer f.finishNumeral(&result, captured)
	v, e := m.normalizeBitVectorWithFrame(n, bits, false, g, &f.normalizationChild)
	if e != nil {
		return false, e
	}
	f.values[0] = v
	f.placed[0], f.complete[0] = true, true
	v, e = m.numerals.PowerOfTwoWithInvocation(bits-1, &f.children[0])
	if e != nil {
		return false, e
	}
	f.values[1] = v
	f.placed[1], f.complete[1] = true, true
	return m.numerals.GreaterEqual(&f.values[0], &f.values[1])
}
