package ast

// Source: src/ast/ast.cpp:1904-1931; concrete sort substitution used by
// retained SMT-LIB parametric sort definitions.

// SubstituteSort simultaneously replaces formal sorts throughout a sort's
// AST-valued parameters. Symbol and numeral parameters are preserved.
func (m *ASTManager) SubstituteSort(sort ASTHandle, source, destination []ASTHandle) (ASTHandle, error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, err
	}
	defer m.mu.Unlock()
	if len(source) != len(destination) {
		return ASTHandle{}, ErrASTDomain
	}
	root, err := m.requireKind(sort, ASTSort)
	if err != nil {
		return ASTHandle{}, err
	}
	from := make([]astNodeRef, len(source))
	to := make([]astNodeRef, len(destination))
	for i := range source {
		from[i], err = m.requireKind(source[i], ASTSort)
		if err != nil {
			return ASTHandle{}, err
		}
		to[i], err = m.requireKind(destination[i], ASTSort)
		if err != nil {
			return ASTHandle{}, err
		}
	}
	result, err := m.datatypeSubstituteSort(root, from, to)
	return m.external(result), err
}
