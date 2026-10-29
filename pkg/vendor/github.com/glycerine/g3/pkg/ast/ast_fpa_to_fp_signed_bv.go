package ast

// Source: src/ast/rewriter/fpa_rewriter.cpp:106-119,155-182.

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// RewriteFPAToFPSignedBVNumeral implements the rounding-mode plus signed
// bit-vector branch of the source's overloaded to_fp reducer.
func (m *ASTManager) RewriteFPAToFPSignedBVNumeral(decl, rm, bv ASTHandle) (out ASTHandle, found bool, result error) {
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
	if family != p.family || kind != astFPAToFP {
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
	// The source constructs its outer scoped_mpf before checking the rounding
	// mode or probing the bit-vector numeral.
	v, err := p.beginNumeralProbe()
	if err != nil {
		return ASTHandle{}, false, err
	}
	defer p.finishNumeralProbe(v, &result, &found)
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
	if result = m.traceFPAToFPRational(177, "r1: ", &r); result != nil {
		return ASTHandle{}, true, result
	}
	qm, err := p.numeric.BorrowRationalManager()
	if err != nil {
		return ASTHandle{}, true, err
	}
	owner := qm.Z
	q := unumeral.NewRational()
	defer func() { result = errors.Join(result, qm.Del(&q)) }()
	if result = owner.CopyFromOwner(&q.Num, m.numerals.Manager.Z, &r.Raw.Num); result != nil {
		return ASTHandle{}, true, result
	}
	// BV numerals are already normalized to [0, 2^width). The signed norm
	// subtracts 2^width exactly when the high bit is set.
	one := unumeral.NewInt(1)
	var half, full unumeral.Int
	defer func() {
		result = errors.Join(result, owner.Del(&full))
		result = errors.Join(result, owner.Del(&half))
	}()
	if result = owner.Mul2k(&one, width-1, &half); result != nil {
		return ASTHandle{}, true, result
	}
	comparison, err := owner.Compare(&q.Num, &half)
	if err != nil {
		return ASTHandle{}, true, err
	}
	if comparison >= 0 {
		if result = owner.Mul2k(&one, width, &full); result != nil {
			return ASTHandle{}, true, result
		}
		if result = owner.Sub(&q.Num, &full, &q.Num); result != nil {
			return ASTHandle{}, true, result
		}
	}
	if result = m.traceFPAToFPRawRational(179, "r1 norm: ", &q); result != nil {
		return ASTHandle{}, true, result
	}
	result = p.numeric.SetRational(&v.value, ebits, sbits, mode, &q)
	if result != nil {
		return ASTHandle{}, true, result
	}
	m.formatExecuting = previous
	node, result = p.mkNumeral(&v.value, nil)
	return ASTHandle{}, true, result
}
