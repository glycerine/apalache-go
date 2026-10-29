package ast

// Source: src/ast/rewriter/fpa_rewriter.cpp:708-763;
// src/util/scoped_numeral.h:24-55.

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// RewriteFPAToBVNumeral rounds a finite FP numeral and checks the declared
// signed or unsigned range before constructing the bit-vector numeral.
func (m *ASTManager) RewriteFPAToBVNumeral(decl, rm, value ASTHandle, highUnspecified bool) (out ASTHandle, found bool, result error) {
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
	if p == nil || p.self != p || !p.published || p.sourceEnded || m.bitVectorPlugin == nil {
		return ASTHandle{}, false, ErrASTHandle
	}
	family, kind, err := m.declIdentity(d)
	if err != nil {
		return ASTHandle{}, false, err
	}
	if family != p.family || kind != astFPAToUBV && kind != astFPAToSBV {
		return ASTHandle{}, false, ErrASTDomain
	}
	info, err := m.funcInfo(m.declAt(d.slot).info)
	if err != nil {
		return ASTHandle{}, false, err
	}
	param, err := m.infoParameter(info, 0)
	if err != nil {
		return ASTHandle{}, false, err
	}
	widthParam, err := m.parameterOfKind(param, ASTParameterInt)
	if err != nil {
		return ASTHandle{}, false, err
	}
	width := uint32(widthParam.integer)
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
	v, err := p.beginNumeralProbe()
	if err != nil {
		return ASTHandle{}, false, err
	}
	defer p.finishNumeralProbe(v, &result, &found)
	hasMode, mode, err := m.readFPARoundingMode(rmRef)
	if err != nil || !hasMode {
		return ASTHandle{}, false, err
	}
	found, result = p.isNumeral(valueRef, &v.value, &v.copy)
	if result != nil || !found {
		return ASTHandle{}, found, result
	}
	unspecified := func() (ASTHandle, bool, error) {
		if !highUnspecified {
			return ASTHandle{}, false, nil
		}
		var e error
		node, e = m.bitVectorNumeralUnsigned(0, width)
		return ASTHandle{}, true, e
	}
	nan, err := p.numeric.IsNaN(&v.value)
	if err != nil {
		return ASTHandle{}, true, err
	}
	inf := false
	if !nan {
		inf, err = p.numeric.IsInf(&v.value)
		if err != nil {
			return ASTHandle{}, true, err
		}
	}
	if nan || inf {
		return unspecified()
	}
	qm, err := p.numeric.BorrowRationalManager()
	if err != nil {
		return ASTHandle{}, true, err
	}
	owner := qm.Z
	q := unumeral.NewRational()
	defer func() { result = errors.Join(result, qm.Del(&q)) }()
	result = p.numeric.ToSBVMPQ(mode, &v.value, &q)
	if result != nil {
		return ASTHandle{}, true, result
	}
	var upper, lower unumeral.Int
	defer func() {
		result = errors.Join(result, owner.Del(&upper))
		result = errors.Join(result, owner.Del(&lower))
	}()
	shift := width
	if kind == astFPAToSBV {
		shift--
	}
	one := unumeral.NewInt(1)
	if result = owner.Mul2k(&one, shift, &upper); result != nil {
		return ASTHandle{}, true, result
	}
	if kind == astFPAToSBV {
		if result = owner.Mul2k(&one, shift, &lower); result != nil {
			return ASTHandle{}, true, result
		}
		if result = owner.Neg(&lower); result != nil {
			return ASTHandle{}, true, result
		}
	}
	if result = owner.Dec(&upper); result != nil {
		return ASTHandle{}, true, result
	}
	lo, err := owner.Compare(&q.Num, &lower)
	if err != nil {
		return ASTHandle{}, true, err
	}
	hi, err := owner.Compare(&q.Num, &upper)
	if err != nil {
		return ASTHandle{}, true, err
	}
	if lo < 0 || hi > 0 {
		return unspecified()
	}
	owned, err := m.numerals.NewValue()
	if err != nil {
		return ASTHandle{}, true, err
	}
	var copied unumeral.RationalServiceInvocation
	defer func() {
		result = errors.Join(result, copied.Cleanup(false))
		result = errors.Join(result, m.numerals.Del(&owned))
		result = errors.Join(result, copied.Cleanup(true))
		result = errors.Join(result, copied.EndContext())
	}()
	result = m.numerals.SetIntegerFromOwnerWithInvocation(&owned, owner, &q.Num, &copied)
	if result != nil {
		return ASTHandle{}, true, result
	}
	node, result = m.mkBitVectorNumeral(&owned, width)
	return ASTHandle{}, true, result
}
