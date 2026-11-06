package ast

// Source: src/ast/rewriter/fpa_rewriter.cpp:106-119,189-213.

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// RewriteFPAToFPExponentRationalNumerals handles both RoundingMode/Real/Int
// and RoundingMode/Int/Real forms of to_fp.
func (m *ASTManager) RewriteFPAToFPExponentRationalNumerals(decl, rm, firstArg, secondArg ASTHandle) (out ASTHandle, found bool, result error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, false, err
	}
	defer m.mu.Unlock()
	d, err := m.requireKind(decl, ASTFuncDecl)
	if err != nil {
		return ASTHandle{}, false, err
	}
	args := [3]ASTHandle{rm, firstArg, secondArg}
	var refs [3]astNodeRef
	for i, arg := range args {
		refs[i], err = m.requireExpr(arg)
		if err != nil {
			return ASTHandle{}, false, err
		}
	}
	p := m.floatingPointPlugin
	if p == nil || p.self != p || !p.published || p.sourceEnded {
		return ASTHandle{}, false, ErrASTHandle
	}
	family, kind, err := m.declIdentity(d)
	if err != nil {
		return ASTHandle{}, false, err
	}
	if family != p.family || kind != astFPAToFP || m.declAt(d.slot).arity != 3 {
		return ASTHandle{}, false, ErrASTDomain
	}
	secondDomain, err := m.readInternal(m.declAt(d.slot).domain, 1)
	if err != nil {
		return ASTHandle{}, false, err
	}
	thirdDomain, err := m.readInternal(m.declAt(d.slot).domain, 2)
	if err != nil {
		return ASTHandle{}, false, err
	}
	if !((secondDomain == p.real && thirdDomain == p.integer) ||
		(secondDomain == p.integer && thirdDomain == p.real)) {
		return ASTHandle{}, false, ErrASTDomain
	}
	info, err := m.funcInfo(m.declAt(d.slot).info)
	if err != nil {
		return ASTHandle{}, false, err
	}
	firstParam, err := m.infoParameter(info, 0)
	if err != nil {
		return ASTHandle{}, false, err
	}
	secondParam, err := m.infoParameter(info, 1)
	if err != nil {
		return ASTHandle{}, false, err
	}
	eParam, err := m.parameterOfKind(firstParam, ASTParameterInt)
	if err != nil {
		return ASTHandle{}, false, err
	}
	sParam, err := m.parameterOfKind(secondParam, ASTParameterInt)
	if err != nil {
		return ASTHandle{}, false, err
	}
	var node astNodeRef
	defer func() {
		if result != nil {
			found = false
			node = astNodeRef{}
		}
		out = m.external(node)
	}()
	// The source constructs the caller-local MPF before checking the mode.
	v, err := p.beginNumeralProbe()
	if err != nil {
		return ASTHandle{}, false, err
	}
	defer p.finishNumeralProbe(v, &result, &found)
	hasMode, mode, err := m.readFPARoundingMode(refs[0])
	if err != nil || !hasMode {
		return ASTHandle{}, false, err
	}
	var values [2]unumeral.OwnedRational
	var copied [2]unumeral.RationalServiceInvocation
	for i := range values {
		values[i], result = m.numerals.NewValue()
		if result != nil {
			return ASTHandle{}, false, result
		}
		defer func(i int) {
			result = errors.Join(result, copied[i].Cleanup(false))
			result = errors.Join(result, m.numerals.Del(&values[i]))
			result = errors.Join(result, copied[i].Cleanup(true))
			result = errors.Join(result, copied[i].EndContext())
		}(i)
		var integer bool
		found, result = m.readArithmeticNumeralWithInvocation(p.arithFamily, refs[i+1], &values[i], &integer, &copied[i])
		if result != nil || !found {
			return ASTHandle{}, found, result
		}
	}
	traceLine := 195
	if secondDomain == p.integer {
		traceLine = 209
	}
	if result = m.traceFPAToFPRationalPair(traceLine, &values[0], &values[1]); result != nil {
		return ASTHandle{}, true, result
	}
	realIndex, integerIndex := 0, 1
	if secondDomain == p.integer {
		realIndex, integerIndex = 1, 0
	}
	qm, err := p.numeric.BorrowRationalManager()
	if err != nil {
		return ASTHandle{}, true, err
	}
	q := unumeral.NewRational()
	var exponent unumeral.Int
	defer func() {
		result = errors.Join(result, qm.Del(&q))
		result = errors.Join(result, qm.Z.Del(&exponent))
	}()
	if result = qm.Z.CopyFromOwner(&q.Num, m.numerals.Manager.Z, &values[realIndex].Raw.Num); result != nil {
		return ASTHandle{}, true, result
	}
	if result = qm.Z.CopyFromOwner(&q.Den, m.numerals.Manager.Z, &values[realIndex].Raw.Den); result != nil {
		return ASTHandle{}, true, result
	}
	if result = qm.Z.CopyFromOwner(&exponent, m.numerals.Manager.Z, &values[integerIndex].Raw.Num); result != nil {
		return ASTHandle{}, true, result
	}
	result = p.numeric.SetExponentRational(&v.value, uint32(eParam.integer), uint32(sParam.integer), mode, &exponent, &q)
	if result != nil {
		return ASTHandle{}, true, result
	}
	node, result = p.mkNumeral(&v.value, nil)
	return ASTHandle{}, true, result
}
