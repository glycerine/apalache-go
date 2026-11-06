package ast

// Source: src/api/api_fpa.cpp:908-1158.

// FPANumeralParts is an owning scalar copy of the retained MPF fields.
type FPANumeralParts struct {
	Sign                     bool
	Exponent                 int64
	ExponentBits             uint32
	SignificandBits          uint32
	Significand              string
	Zero, Infinite, Denormal bool
}

// ReadFPANumeralParts copies the raw numeral fields while the source scoped
// MPF is live. NaN and symbolic floating-point expressions are rejected.
func (m *ASTManager) ReadFPANumeralParts(value ASTHandle) (parts FPANumeralParts, matched bool, result error) {
	if result = m.lock(); result != nil {
		return parts, false, result
	}
	defer m.mu.Unlock()
	ref, err := m.requireExpr(value)
	if err != nil {
		return parts, false, err
	}
	plugin := m.floatingPointPlugin
	if plugin == nil || plugin.sourceEnded {
		return parts, false, ErrASTHandle
	}
	frame, err := plugin.beginNumeralProbe()
	if err != nil {
		return parts, false, err
	}
	defer plugin.finishNumeralProbe(frame, &result, &matched)
	matched, result = plugin.isNumeral(ref, &frame.value, &frame.copy)
	if result != nil || !matched {
		return parts, matched, result
	}
	nan, err := plugin.numeric.IsNaN(&frame.value)
	if err != nil {
		return parts, false, err
	}
	if nan {
		return parts, false, ErrASTDomain
	}
	parts.Sign = frame.value.Sign()
	parts.Exponent = frame.value.Exponent()
	parts.ExponentBits = frame.value.EBits()
	parts.SignificandBits = frame.value.SBits()
	parts.Zero, err = plugin.numeric.IsZero(&frame.value)
	if err != nil {
		return parts, false, err
	}
	parts.Infinite, err = plugin.numeric.IsInf(&frame.value)
	if err != nil {
		return parts, false, err
	}
	parts.Denormal, err = plugin.numeric.IsDenormal(&frame.value)
	if err != nil {
		return parts, false, err
	}
	integer, err := plugin.numeric.BorrowIntegerManager()
	if err != nil {
		return parts, false, err
	}
	significand, err := plugin.numeric.BorrowSignificand(&frame.value)
	if err != nil {
		return parts, false, err
	}
	parts.Significand, err = integer.String(significand)
	return parts, err == nil, err
}
