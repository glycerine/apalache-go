package ast

// Source: src/api/api_numeral.cpp:194-226.

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// FPANumeralString reads the API numeral-string fallback after its rational
// probe has failed. Source constructs a scoped MPF before checking either a
// rounding-mode constant or a floating-point numeral.
func (m *ASTManager) FPANumeralString(h ASTHandle) (value string, matched bool, result error) {
	if result = m.lock(); result != nil {
		return
	}
	defer func() {
		if result != nil {
			value = ""
		}
	}()
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return "", false, err
	}
	p := m.floatingPointPlugin
	if p == nil || p.self != p || !p.published || p.sourceEnded {
		return "", false, ErrASTHandle
	}
	f, err := p.beginNumeralProbe()
	if err != nil {
		return "", false, err
	}
	defer p.finishNumeralProbe(f, &result, &matched)
	var mode unumeral.ArbitraryFloatRoundingMode
	matched, mode, result = m.readFPARoundingMode(r)
	if result != nil {
		return
	}
	if matched {
		switch mode {
		case unumeral.ArbitraryFloatNearestEven:
			return "roundNearestTiesToEven", true, nil
		case unumeral.ArbitraryFloatNearestAway:
			return "roundNearestTiesToAway", true, nil
		case unumeral.ArbitraryFloatTowardPositive:
			return "roundTowardPositive", true, nil
		case unumeral.ArbitraryFloatTowardNegative:
			return "roundTowardNegative", true, nil
		default:
			return "roundTowardZero", true, nil
		}
	}
	matched, result = p.isNumeral(r, &f.value, &f.copy)
	if result != nil || !matched {
		return "", matched, result
	}
	text, err := unumeral.NewArbitraryFloatFormatText(m.BudgetField)
	if err != nil {
		return "", true, err
	}
	defer func() { result = errors.Join(result, text.Close()) }()
	view, err := p.numeric.ToStringInto(&f.value, text)
	if err != nil {
		return "", true, err
	}
	n, err := view.Length()
	if err != nil {
		return "", true, err
	}
	if n > uint64(int(^uint(0)>>1)) {
		return "", true, ErrASTDomain
	}
	buf := make([]byte, int(n))
	for i := range buf {
		buf[i], err = view.Byte(uint64(i))
		if err != nil {
			return "", true, err
		}
	}
	return string(buf), true, nil
}
