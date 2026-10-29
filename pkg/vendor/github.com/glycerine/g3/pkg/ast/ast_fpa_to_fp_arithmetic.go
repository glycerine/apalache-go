package ast

// Source: src/ast/rewriter/fpa_rewriter.cpp:106-119,154-165.

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// RewriteFPAToFPArithmeticNumeral implements RoundingMode plus Real or Int
// conversion through the source MPF rational setter.
func (m *ASTManager) RewriteFPAToFPArithmeticNumeral(decl, rm, value ASTHandle) (out ASTHandle, found bool, result error) {
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
	valueRef, err := m.requireExpr(value)
	if err != nil {
		return ASTHandle{}, false, err
	}
	p := m.floatingPointPlugin
	if p == nil || p.self != p || !p.published || p.sourceEnded {
		return ASTHandle{}, false, ErrASTHandle
	}
	family, kind, err := m.declIdentity(d)
	if err != nil {
		return ASTHandle{}, false, err
	}
	if family != p.family || kind != astFPAToFP {
		return ASTHandle{}, false, ErrASTDomain
	}
	domain, err := m.readInternal(m.declAt(d.slot).domain, 1)
	if err != nil {
		return ASTHandle{}, false, err
	}
	if domain != p.real && domain != p.integer {
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
	var node astNodeRef
	defer func() {
		if result != nil {
			found = false
			node = astNodeRef{}
		}
		out = m.external(node)
	}()
	// The source outer MPF is constructed before the rounding-mode guard.
	outer, err := p.beginNumeralProbe()
	if err != nil {
		return ASTHandle{}, false, err
	}
	defer p.finishNumeralProbe(outer, &result, &found)
	hasMode, mode, err := m.readFPARoundingMode(rmRef)
	if err != nil || !hasMode {
		return ASTHandle{}, false, err
	}
	r, err := m.numerals.NewValue()
	if err != nil {
		return ASTHandle{}, false, err
	}
	var copied unumeral.RationalServiceInvocation
	defer func() {
		result = errors.Join(result, copied.Cleanup(false))
		result = errors.Join(result, m.numerals.Del(&r))
		result = errors.Join(result, copied.Cleanup(true))
		result = errors.Join(result, copied.EndContext())
	}()
	var integer bool
	found, result = m.readArithmeticNumeralWithInvocation(p.arithFamily, valueRef, &r, &integer, &copied)
	if result != nil || !found {
		return ASTHandle{}, found, result
	}
	if result = m.traceFPAToFPArithmeticInput(&r); result != nil {
		return ASTHandle{}, true, result
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
	if result = qm.Z.CopyFromOwner(&q.Den, m.numerals.Manager.Z, &r.Raw.Den); result != nil {
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
	node, result = p.mkNumeral(&v.value, nil)
	return ASTHandle{}, true, result
}
