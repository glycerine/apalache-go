package ast

// Source: src/ast/ast.h:1531-1532 ast_manager::mk_family_id.

// EnsureFamilyID returns the existing family identity for name or creates the
// next identity. It does not install a declaration plugin. This is the public
// form of the source operation used by retained clients such as Duality.
func (m *ASTManager) EnsureFamilyID(name string) (FamilyID, error) {
	if err := m.lock(); err != nil {
		return NullFamilyID, err
	}
	defer m.mu.Unlock()
	symbol, err := m.Symbols.Intern(name)
	if err != nil {
		return NullFamilyID, err
	}
	return m.families.MkID(symbol)
}
