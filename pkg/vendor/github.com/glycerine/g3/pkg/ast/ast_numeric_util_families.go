package ast

// Source: src/ast/arith_decl_plugin.cpp:641-645;
// src/ast/bv_decl_plugin.cpp:850-855.

// ArithmeticFamilyID follows arith_util's constructor. It creates only the
// family identity; the utility's declaration-plugin pointer is initially null.
func (m *ASTManager) ArithmeticFamilyID() (FamilyID, error) {
	if e := m.lock(); e != nil {
		return NullFamilyID, e
	}
	defer m.mu.Unlock()
	name, e := m.Symbols.Intern("arith")
	if e != nil {
		return NullFamilyID, e
	}
	return m.families.MkID(name)
}

// BitVectorUtilityFamilyID follows bv_util's reached family lookups and DEBUG
// plugin assertion, independently of installing a declaration plugin.
func (m *ASTManager) BitVectorUtilityFamilyID() (FamilyID, error) {
	if e := m.lock(); e != nil {
		return NullFamilyID, e
	}
	defer m.mu.Unlock()
	name, e := m.Symbols.Intern("bv")
	if e != nil {
		return NullFamilyID, e
	}
	family, e := m.families.MkID(name)
	if e != nil {
		return NullFamilyID, e
	}
	if ASTSourceDebug && !m.pluginAt(family).present {
		return NullFamilyID, ErrASTDomain
	}
	// Source repeats mk_family_id while obtaining the borrowed plugin pointer.
	name, e = m.Symbols.Intern("bv")
	if e != nil {
		return NullFamilyID, e
	}
	return m.families.MkID(name)
}
