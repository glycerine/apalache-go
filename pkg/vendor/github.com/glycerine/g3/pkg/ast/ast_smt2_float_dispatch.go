package ast

// Source: src/ast/ast_smt2_pp.cpp:625-636; src/ast/fpa_decl_plugin.h.

// The source constant dispatch probes a floating-point numeral before it
// chooses pp_float_literal. Keep the probe on the owning plugin so special
// values and finite values follow the same numeral lifecycle.
func (m *ASTManager) smt2IsFloatingPointNumeral(h ASTHandle) (bool, error) {
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
		return false, nil
	}
	return p.isNumeralOnly(r)
}
