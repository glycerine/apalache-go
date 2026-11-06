package rel

import "github.com/glycerine/g3/pkg/ast"

// CanHandleTableSorts mirrors table_relation_plugin::can_handle_signature for
// the concrete finite HashTable specialization.
func (m *RelationManager) CanHandleTableSorts(sorts []ast.ASTHandle) bool {
	_, err := tableRelationSignature(m, sorts)
	return err == nil
}

// CanHandleTablePredicate applies the same capability check to a declaration.
func (m *RelationManager) CanHandleTablePredicate(predicate ast.ASTHandle) bool {
	if m == nil || m.open() != nil {
		return false
	}
	sorts, _, err := m.PredicateSignature(predicate)
	return err == nil && m.CanHandleTableSorts(sorts)
}
