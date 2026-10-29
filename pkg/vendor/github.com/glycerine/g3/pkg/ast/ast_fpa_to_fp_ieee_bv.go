package ast

// Source: src/ast/rewriter/fpa_rewriter.cpp:106-153.

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// RewriteFPAToFPIEEEBVNumeral interprets a concrete BV as the stored FP
// sign, exponent, and significand fields for the declared target format.
func (m *ASTManager) RewriteFPAToFPIEEEBVNumeral(decl, bv ASTHandle) (out ASTHandle, found bool, result error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, false, err
	}
	defer m.mu.Unlock()
	d, err := m.requireKind(decl, ASTFuncDecl)
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
	if family != p.family || kind != astFPAToFP || m.declAt(d.slot).arity != 1 {
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
	// The source outer scoped MPF exists before probing the BV operand.
	v, err := p.beginNumeralProbe()
	if err != nil {
		return ASTHandle{}, false, err
	}
	defer p.finishNumeralProbe(v, &result, &found)
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
	owner, err := p.numeric.BorrowIntegerManager()
	if err != nil {
		return ASTHandle{}, true, err
	}
	one := unumeral.NewInt(1)
	var sig, exp, z, sigPower, expPower unumeral.Int
	defer func() {
		result = errors.Join(result, owner.Del(&z))
		result = errors.Join(result, owner.Del(&exp))
		result = errors.Join(result, owner.Del(&sig))
		result = errors.Join(result, owner.Del(&expPower))
		result = errors.Join(result, owner.Del(&sigPower))
	}()
	if result = owner.Mul2k(&one, sbits-1, &sigPower); result != nil {
		return ASTHandle{}, true, result
	}
	if result = owner.Mul2k(&one, ebits, &expPower); result != nil {
		return ASTHandle{}, true, result
	}
	if result = owner.CopyFromOwner(&z, m.numerals.Manager.Z, &r.Raw.Num); result != nil {
		return ASTHandle{}, true, result
	}
	if result = owner.Rem(&z, &sigPower, &sig); result != nil {
		return ASTHandle{}, true, result
	}
	if result = owner.Div(&z, &sigPower, &z); result != nil {
		return ASTHandle{}, true, result
	}
	if result = owner.Rem(&z, &expPower, &exp); result != nil {
		return ASTHandle{}, true, result
	}
	if result = owner.Div(&z, &expPower, &z); result != nil {
		return ASTHandle{}, true, result
	}
	biasedExp, err := owner.Int64(&exp)
	if err != nil {
		return ASTHandle{}, true, err
	}
	exponent, err := p.numeric.UnbiasExp(ebits, biasedExp)
	if err != nil {
		return ASTHandle{}, true, err
	}
	signFieldZero, err := owner.IsZero(&z)
	if err != nil {
		return ASTHandle{}, true, err
	}
	result = p.numeric.SetComponents(&v.value, ebits, sbits, !signFieldZero, exponent, &sig)
	if result != nil {
		return ASTHandle{}, true, result
	}
	if result = m.traceFPAToFPIEEEBVFields(!signFieldZero, &sig, exponent, &v.value); result != nil {
		return ASTHandle{}, true, result
	}
	m.formatExecuting = previous
	node, result = p.mkNumeral(&v.value, nil)
	return ASTHandle{}, true, result
}
