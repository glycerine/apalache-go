package ast

// Source: src/ast/rewriter/fpa_rewriter.cpp:764-792;
// src/util/scoped_numeral.h:24-55.

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// RewriteFPAToIEEEBVNumeral converts non-NaN MPF numerals to their stored
// bit pattern. In hardware-unspecified mode, NaN becomes the source concat
// expression and requests one more rewrite pass.
func (m *ASTManager) RewriteFPAToIEEEBVNumeral(h ASTHandle, highUnspecified bool) (out ASTHandle, found, rewriteOne bool, result error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, false, false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return ASTHandle{}, false, false, err
	}
	p := m.floatingPointPlugin
	if p == nil || p.self != p || !p.published || p.sourceEnded || m.bitVectorPlugin == nil {
		return ASTHandle{}, false, false, ErrASTHandle
	}
	// Source line 765 precedes construction of the scoped MPF. SMT2 printing
	// acquires this manager's lock, so release it for that source trace body.
	m.mu.Unlock()
	traceErr := m.traceFPAToIEEEBVArgument(h)
	m.mu.Lock()
	if traceErr != nil {
		return ASTHandle{}, false, false, traceErr
	}
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	var node astNodeRef
	defer func() {
		if result != nil {
			found = false
			rewriteOne = false
			node = astNodeRef{}
		}
		out = m.external(node)
	}()
	v, err := p.beginNumeralProbe()
	if err != nil {
		return ASTHandle{}, false, false, err
	}
	defer p.finishNumeralProbe(v, &result, &found)
	found, result = p.isNumeral(r, &v.value, &v.copy)
	if result != nil || !found {
		return ASTHandle{}, found, false, result
	}
	if result = m.traceFPAToIEEEBVNumeral(&v.value); result != nil {
		return ASTHandle{}, true, false, result
	}
	nan, err := p.numeric.IsNaN(&v.value)
	if err != nil {
		return ASTHandle{}, true, false, err
	}
	ebits, sbits := v.value.EBits(), v.value.SBits()
	if nan {
		if !highUnspecified {
			return ASTHandle{}, false, false, nil
		}
		parts := [4]ASTHandle{}
		first, err := m.bitVectorNumeralUnsigned(0, 1)
		if err != nil {
			return ASTHandle{}, true, false, err
		}
		parts[0] = m.external(first)
		one, err := m.bitVectorNumeralUnsigned(1, ebits)
		if err != nil {
			return ASTHandle{}, true, false, err
		}
		negated, err := m.mkFamilyApp(m.bitVectorPlugin.family, 3, nil, []ASTHandle{m.external(one)})
		if err != nil {
			return ASTHandle{}, true, false, err
		}
		parts[1] = m.external(negated)
		third, err := m.bitVectorNumeralUnsigned(0, sbits-2)
		if err != nil {
			return ASTHandle{}, true, false, err
		}
		parts[2] = m.external(third)
		last, err := m.bitVectorNumeralUnsigned(1, 1)
		if err != nil {
			return ASTHandle{}, true, false, err
		}
		parts[3] = m.external(last)
		node, result = m.mkFamilyApp(m.bitVectorPlugin.family, 37, nil, parts[:])
		return ASTHandle{}, true, true, result
	}
	owner, err := p.numeric.BorrowIntegerManager()
	if err != nil {
		return ASTHandle{}, true, false, err
	}
	var bits unumeral.Int
	defer func() { result = errors.Join(result, owner.Del(&bits)) }()
	result = p.numeric.ToIEEEBVMPZ(&v.value, &bits)
	if result != nil {
		return ASTHandle{}, true, false, result
	}
	q, err := m.numerals.NewValue()
	if err != nil {
		return ASTHandle{}, true, false, err
	}
	var copied unumeral.RationalServiceInvocation
	defer func() {
		result = errors.Join(result, copied.Cleanup(false))
		result = errors.Join(result, m.numerals.Del(&q))
		result = errors.Join(result, copied.Cleanup(true))
		result = errors.Join(result, copied.EndContext())
	}()
	result = m.numerals.SetIntegerFromOwnerWithInvocation(&q, owner, &bits, &copied)
	if result != nil {
		return ASTHandle{}, true, false, result
	}
	node, result = m.mkBitVectorNumeral(&q, ebits+sbits)
	return ASTHandle{}, true, false, result
}
