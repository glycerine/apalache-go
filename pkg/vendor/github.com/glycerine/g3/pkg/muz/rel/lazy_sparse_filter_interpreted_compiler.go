package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

func copyLazySparseColumns(source map[uint32]int) map[uint32]int {
	result := make(map[uint32]int, len(source))
	for variable, column := range source {
		result[variable] = column
	}
	return result
}

func copyLazySparseSorts(source map[uint32]ast.ASTHandle) map[uint32]ast.ASTHandle {
	result := make(map[uint32]ast.ASTHandle, len(source))
	for variable, sort := range source {
		result[variable] = sort
	}
	return result
}

// filterInterpretedCompiled appends a deferred compiler guard and owns copies
// of every condition until the lazy node is finally released.
func (t *LazySparseTable) filterInterpretedCompiled(manager *RelationManager, rewriter *astrewriter.TheoryRewriter, conditions []ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle) error {
	if err := t.open(); err != nil || manager == nil || manager.open() != nil || rewriter == nil || rewriter.Manager() != manager.manager || len(conditions) == 0 {
		return errors.Join(ErrLazySparse, err)
	}
	for variable, sort := range sorts {
		column, found := columns[variable]
		if !found || column < 0 || column >= t.root.signature.Len() {
			return ErrLazySparse
		}
		domain, err := manager.sortDomain(sort)
		if err != nil || domain != t.root.signature.domains[column] {
			return errors.Join(ErrLazySparse, err)
		}
	}
	owner := &lazySparseConditionOwner{engine: t.engine, manager: manager.manager}
	t.engine.conditions = append(t.engine.conditions, owner)
	for _, condition := range conditions {
		reference, err := manager.manager.NewRef(condition)
		if err != nil {
			_, cleanupErr := owner.release()
			return errors.Join(err, cleanupErr)
		}
		owner.refs = append(owner.refs, reference)
	}
	payload := &lazySparseInterpretedFilter{
		owner: owner, manager: manager, rewriter: rewriter,
		columns: copyLazySparseColumns(columns), sorts: copyLazySparseSorts(sorts),
	}
	if err := t.replaceLazySparseRoot(&lazySparseNode{
		kind: LazyTableFilterInterpreted, signature: Signature{domains: t.root.signature.Domains()}, functional: t.root.functional,
		left: t.root, interpreted: payload,
	}); err != nil {
		_, cleanupErr := owner.release()
		return errors.Join(err, cleanupErr)
	}
	return nil
}
