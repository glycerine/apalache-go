package ast

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (m *ASTManager) parameterOfKind(p ASTParameter, kind ASTParameterKind) (*astParameterRecord, error) {
	r, e := m.parameter(p)
	if e != nil {
		return nil, e
	}
	if r.Kind != kind {
		return nil, ErrASTDomain
	}
	return r, nil
}
func (m *ASTManager) ParameterInt(p ASTParameter) (int32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.parameterOfKind(p, ASTParameterInt)
	if e != nil {
		return 0, e
	}
	return r.integer, nil
}
func (m *ASTManager) ParameterAST(p ASTParameter) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.parameterOfKind(p, ASTParameterAST)
	if e != nil {
		return ASTHandle{}, e
	}
	return m.external(r.ast), nil
}
func (m *ASTManager) ParameterSymbol(p ASTParameter) (usymbol.Symbol, error) {
	if e := m.lock(); e != nil {
		return usymbol.Symbol{}, e
	}
	defer m.mu.Unlock()
	r, e := m.parameterOfKind(p, ASTParameterSymbol)
	if e != nil {
		return usymbol.Symbol{}, e
	}
	return r.symbol, nil
}
func (m *ASTManager) ParameterDouble(p ASTParameter) (float64, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.parameterOfKind(p, ASTParameterDouble)
	if e != nil {
		return 0, e
	}
	return r.double, nil
}
func (m *ASTManager) ParameterExternal(p ASTParameter) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.parameterOfKind(p, ASTParameterExternal)
	if e != nil {
		return 0, e
	}
	return r.external, nil
}

// ParameterRationalCopy is the source is_rational(rational&) overload:
// a kind mismatch leaves the caller's owner-confined numeral untouched.
// The C++ const-reference getter is not exposed as a second owning Go token.
func (m *ASTManager) ParameterRationalCopy(p ASTParameter, out *unumeral.OwnedRational) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.parameter(p)
	if e != nil {
		return false, e
	}
	if r.Kind != ASTParameterRational {
		return false, nil
	}
	if e = m.numerals.Copy(out, &r.Rational); e != nil {
		return false, e
	}
	return true, nil
}
