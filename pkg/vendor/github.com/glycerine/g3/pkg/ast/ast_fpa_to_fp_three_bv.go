package ast

// Source: src/ast/rewriter/fpa_rewriter.cpp:106-119,212-230.

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// RewriteFPAToFPThreeBVNumerals assembles concrete sign, biased exponent,
// and significand bit-vectors into a floating-point numeral.
func (m *ASTManager) RewriteFPAToFPThreeBVNumerals(decl, sign, exponent, significand ASTHandle) (out ASTHandle, found bool, result error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, false, err
	}
	defer m.mu.Unlock()
	d, err := m.requireKind(decl, ASTFuncDecl)
	if err != nil {
		return ASTHandle{}, false, err
	}
	args := [3]ASTHandle{sign, exponent, significand}
	var refs [3]astNodeRef
	for i, arg := range args {
		refs[i], err = m.requireExpr(arg)
		if err != nil {
			return ASTHandle{}, false, err
		}
	}
	p := m.floatingPointPlugin
	if p == nil || p.self != p || !p.published || p.sourceEnded || m.bitVectorPlugin == nil {
		return ASTHandle{}, false, ErrASTHandle
	}
	family, kind, err := m.declIdentity(d)
	if err != nil {
		return ASTHandle{}, false, err
	}
	if family != p.family || kind != astFPAToFP || m.declAt(d.slot).arity != 3 {
		return ASTHandle{}, false, ErrASTDomain
	}
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
	var values [3]unumeral.OwnedRational
	var widths [3]uint32
	for i, ref := range refs {
		values[i], result = m.numerals.NewValue()
		if result != nil {
			return ASTHandle{}, false, result
		}
		defer func(i int) { result = errors.Join(result, m.numerals.Del(&values[i])) }(i)
		found, result = m.readBitVectorNumeral(ref, &values[i], &widths[i])
		if result != nil || !found {
			return ASTHandle{}, found, result
		}
	}
	negative, err := m.numerals.IsOne(&values[0])
	if err != nil {
		return ASTHandle{}, true, err
	}
	owner, err := p.numeric.BorrowIntegerManager()
	if err != nil {
		return ASTHandle{}, true, err
	}
	var biased, sig unumeral.Int
	defer func() {
		result = errors.Join(result, owner.Del(&sig))
		result = errors.Join(result, owner.Del(&biased))
	}()
	if result = owner.CopyFromOwner(&biased, m.numerals.Manager.Z, &values[1].Raw.Num); result != nil {
		return ASTHandle{}, true, result
	}
	if result = owner.CopyFromOwner(&sig, m.numerals.Manager.Z, &values[2].Raw.Num); result != nil {
		return ASTHandle{}, true, result
	}
	biasedExp, err := owner.Int64(&biased)
	if err != nil {
		return ASTHandle{}, true, err
	}
	exp, err := p.numeric.UnbiasExp(widths[1], biasedExp)
	if err != nil {
		return ASTHandle{}, true, err
	}
	result = p.numeric.SetComponents(&v.value, widths[1], widths[2]+1, negative, exp, &sig)
	if result != nil {
		return ASTHandle{}, true, result
	}
	if result = m.traceFPAToFPValue(226, "v = ", &v.value); result != nil {
		return ASTHandle{}, true, result
	}
	m.formatExecuting = previous
	node, result = p.mkNumeral(&v.value, nil)
	return ASTHandle{}, true, result
}
