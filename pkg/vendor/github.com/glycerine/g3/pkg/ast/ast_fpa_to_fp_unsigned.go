package ast

// Source: src/ast/rewriter/fpa_rewriter.cpp:235-254.

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// RewriteFPAToFPUnsignedNumeral converts a concrete unsigned bit-vector using
// the source rounding mode and the existing MPF rational conversion.
func (m *ASTManager) RewriteFPAToFPUnsignedNumeral(decl, rm, bv ASTHandle) (out ASTHandle, found bool, result error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, false, err
	}
	defer m.mu.Unlock()
	d, err := m.requireKind(decl, ASTFuncDecl)
	if err != nil {
		return ASTHandle{}, false, err
	}
	rmRef, err := m.requireExpr(rm)
	if err != nil {
		return ASTHandle{}, false, err
	}
	bvRef, err := m.requireExpr(bv)
	if err != nil {
		return ASTHandle{}, false, err
	}
	p := m.floatingPointPlugin
	if p == nil || p.self != p || !p.published || p.sourceEnded || m.bitVectorPlugin == nil {
		return ASTHandle{}, false, ErrASTHandle
	}
	family, kind, err := m.declIdentity(d)
	if err != nil {
		return ASTHandle{}, false, err
	}
	if family != p.family || kind != astFPAToFPUnsigned {
		return ASTHandle{}, false, ErrASTDomain
	}
	info, err := m.funcInfo(m.declAt(d.slot).info)
	if err != nil {
		return ASTHandle{}, false, err
	}
	first, err := m.infoParameter(info, 0)
	if err != nil {
		return ASTHandle{}, false, err
	}
	second, err := m.infoParameter(info, 1)
	if err != nil {
		return ASTHandle{}, false, err
	}
	eParam, err := m.parameterOfKind(first, ASTParameterInt)
	if err != nil {
		return ASTHandle{}, false, err
	}
	sParam, err := m.parameterOfKind(second, ASTParameterInt)
	if err != nil {
		return ASTHandle{}, false, err
	}
	ebits, sbits := uint32(eParam.integer), uint32(sParam.integer)
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	var node astNodeRef
	defer func() {
		if result != nil {
			found = false
			node = astNodeRef{}
		}
		out = m.external(node)
	}()
	hasMode, mode, err := m.readFPARoundingMode(rmRef)
	if err != nil || !hasMode {
		return ASTHandle{}, false, err
	}
	r, err := m.numerals.NewValue()
	if err != nil {
		return ASTHandle{}, false, err
	}
	defer func() { result = errors.Join(result, m.numerals.Del(&r)) }()
	var width uint32
	found, result = m.readBitVectorNumeral(bvRef, &r, &width)
	if result != nil || !found {
		return ASTHandle{}, found, result
	}
	qm, err := p.numeric.BorrowRationalManager()
	if err != nil {
		return ASTHandle{}, true, err
	}
	q := unumeral.NewRational()
	defer func() { result = errors.Join(result, qm.Del(&q)) }()
	if result = qm.Z.CopyFromOwner(&q.Num, m.numerals.Manager.Z, &r.Raw.Num); result != nil {
		return ASTHandle{}, true, result
	}
	v, err := p.beginNumeralProbe()
	if err != nil {
		return ASTHandle{}, true, err
	}
	defer p.finishNumeralProbe(v, &result, &found)
	result = p.numeric.SetRational(&v.value, ebits, sbits, mode, &q)
	if result != nil {
		return ASTHandle{}, true, result
	}
	m.formatExecuting = previous
	node, result = p.mkNumeral(&v.value, nil)
	return ASTHandle{}, true, result
}
