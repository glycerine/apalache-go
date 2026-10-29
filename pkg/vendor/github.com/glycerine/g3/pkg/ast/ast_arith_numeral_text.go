package ast

// Source: src/ast/arith_decl_plugin.cpp:632-639.

// ArithmeticNumeralText reads the exact numeral value and integer flag for
// the retained arithmetic-tactic consumer. The returned Go string is detached
// from the AST parameter's numeral owner.
func (m *ASTManager) ArithmeticNumeralText(h ASTHandle) (value string, integer, matched bool, err error) {
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return
	}
	if m.nodeAt(r.slot).kind != ASTApp || m.arithmeticPlugin == nil {
		return
	}
	family, kind, err := m.declIdentity(m.appAt(r.slot).decl)
	if err != nil || family != m.arithmeticPlugin.family || kind != 0 {
		return
	}
	info, err := m.funcInfo(m.declAt(m.appAt(r.slot).decl.slot).info)
	if err != nil {
		return
	}
	p, err := m.infoParameter(info, 0)
	if err != nil {
		return
	}
	first, err := m.parameter(p)
	if err != nil {
		return
	}
	if first.Kind != ASTParameterRational {
		err = ErrASTDomain
		return
	}
	value, err = m.numerals.String(&first.Rational)
	if err != nil {
		return
	}
	p, err = m.infoParameter(info, 1)
	if err != nil {
		return
	}
	second, err := m.parameter(p)
	if err != nil {
		return
	}
	if second.Kind != ASTParameterInt {
		err = ErrASTDomain
		return
	}
	integer = second.integer != 0
	return value, integer, true, nil
}
