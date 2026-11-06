package ast

// Source: src/ast/rewriter/fpa_rewriter.cpp:547-638;
// src/ast/fpa_decl_plugin.cpp:101-139; src/util/scoped_numeral.h:24-33.

// FPAClassificationKind is one of the seven retained unary FPA relation
// kinds. These values match fpa_op_kind in fpa_decl_plugin.h.
type FPAClassificationKind int32

const (
	FPAIsNaN       FPAClassificationKind = FPAClassificationKind(astFPAIsNaN)
	FPAIsInfinite  FPAClassificationKind = FPAClassificationKind(astFPAIsInf)
	FPAIsZero      FPAClassificationKind = FPAClassificationKind(astFPAIsZero)
	FPAIsNormal    FPAClassificationKind = FPAClassificationKind(astFPAIsNormal)
	FPAIsSubnormal FPAClassificationKind = FPAClassificationKind(astFPAIsSubnormal)
	FPAIsNegative  FPAClassificationKind = FPAClassificationKind(astFPAIsNegative)
	FPAIsPositive  FPAClassificationKind = FPAClassificationKind(astFPAIsPositive)
)

// ClassifyFPANumeral owns the source rewriter's one scoped_mpf until the
// predicate completes. A symbolic operand returns found=false. The AST
// plugin retains a stopped scalar destructor for its owning finalization.
func (m *ASTManager) ClassifyFPANumeral(h ASTHandle, kind FPAClassificationKind) (found, value bool, result error) {
	if err := m.lock(); err != nil {
		return false, false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return false, false, err
	}
	p := m.floatingPointPlugin
	if p == nil || p.self != p || !p.published || p.sourceEnded {
		return false, false, ErrASTHandle
	}
	if kind < FPAIsNaN || kind > FPAIsPositive {
		return false, false, ErrASTDomain
	}
	f, err := p.beginNumeralProbe()
	if err != nil {
		return false, false, err
	}
	defer p.finishNumeralProbe(f, &result, nil)
	found, result = p.isNumeral(r, &f.value, &f.copy)
	if result != nil || !found {
		return found, false, result
	}
	switch kind {
	case FPAIsNaN:
		value, result = p.numeric.IsNaN(&f.value)
	case FPAIsInfinite:
		value, result = p.numeric.IsInf(&f.value)
	case FPAIsZero:
		value, result = p.numeric.IsZero(&f.value)
	case FPAIsNormal:
		value, result = p.numeric.IsNormal(&f.value)
	case FPAIsSubnormal:
		value, result = p.numeric.IsDenormal(&f.value)
	case FPAIsNegative:
		value, result = p.numeric.IsNegative(&f.value)
	case FPAIsPositive:
		var negative bool
		negative, result = p.numeric.IsNegative(&f.value)
		if result == nil && !negative {
			var nan bool
			nan, result = p.numeric.IsNaN(&f.value)
			value = !nan
		}
	}
	return found, value, result
}

// IsFPANegativeInfinity and IsFPAPositiveInfinity are the source fpa_util
// special-value guards reached before mk_lt's two-numeral comparison.
func (m *ASTManager) IsFPANegativeInfinity(h ASTHandle) (bool, error) {
	return m.fpaIsInfinitySign(h, true)
}

func (m *ASTManager) IsFPAPositiveInfinity(h ASTHandle) (bool, error) {
	return m.fpaIsInfinitySign(h, false)
}

func (m *ASTManager) fpaIsInfinitySign(h ASTHandle, negative bool) (bool, error) {
	if err := m.lock(); err != nil {
		return false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return false, err
	}
	p := m.floatingPointPlugin
	if p == nil || p.self != p || !p.published || p.sourceEnded {
		return false, ErrASTHandle
	}
	if negative {
		return p.isNumeralClass(r, astFloatNInf)
	}
	return p.isNumeralClass(r, astFloatPInf)
}
