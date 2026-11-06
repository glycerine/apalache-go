package rel

// Source: src/muz/rel/dl_lazy_table.cpp lazy_table_filter_interpreted.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

type lazyInterpretedFilter struct {
	owner         *lazyConditionOwner
	manager       *RelationManager
	rewriter      *astrewriter.TheoryRewriter
	relationSorts []ast.ASTHandle
	columns       map[uint32]int
	sorts         map[uint32]ast.ASTHandle
}

func (f *lazyInterpretedFilter) handles() ([]ast.ASTHandle, error) {
	if f == nil || f.owner == nil || f.owner.manager == nil || f.rewriter == nil {
		return nil, ErrLazyTable
	}
	if f.rewriter.Manager() != f.owner.manager {
		return nil, ErrLazyTable
	}
	conditions := make([]ast.ASTHandle, len(f.owner.refs))
	for index, reference := range f.owner.refs {
		handle, err := f.owner.manager.RefGet(reference)
		if err != nil {
			return nil, err
		}
		conditions[index] = handle
	}
	return conditions, nil
}

// FilterInterpreted appends a deferred finite-table condition. The lazy node
// owns references to its AST conditions until the last handle retaining that
// node is closed, as lazy_table_filter_interpreted owns an app_ref.
func (t *LazyTable) FilterInterpreted(manager *RelationManager, relationSorts []ast.ASTHandle, rewriter *astrewriter.TheoryRewriter, conditions []ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle) error {
	if err := t.open(); err != nil {
		return err
	}
	if manager == nil || manager.open() != nil || rewriter == nil || rewriter.Manager() != manager.manager || len(conditions) == 0 {
		return ErrLazyTable
	}
	signature, err := tableRelationSignature(manager, relationSorts)
	if err != nil {
		return err
	}
	if !signature.equal(t.root.signature) {
		return ErrLazyTable
	}
	proxy := &TableRelation{manager: manager, sorts: append([]ast.ASTHandle(nil), relationSorts...), table: NewHashTable(signature)}
	defer proxy.table.Close()
	if err = proxy.validateInterpretedFilter(rewriter, conditions, columns, sorts); err != nil {
		return err
	}
	for _, condition := range conditions {
		kind, kindErr := manager.manager.GetKind(condition)
		if kindErr != nil {
			return kindErr
		}
		if kind != ast.ASTApp {
			return fmt.Errorf("%w: source interpreted condition must be an application", ErrLazyTable)
		}
	}
	owner := &lazyConditionOwner{engine: t.engine, manager: manager.manager}
	t.engine.conditions = append(t.engine.conditions, owner)
	for _, condition := range conditions {
		reference, refErr := manager.manager.NewRef(condition)
		if refErr != nil {
			complete, cleanupErr := owner.release()
			if !complete {
				return errors.Join(refErr, cleanupErr)
			}
			return refErr
		}
		owner.refs = append(owner.refs, reference)
	}
	filter := &lazyInterpretedFilter{
		owner: owner, manager: manager, rewriter: rewriter,
		relationSorts: append([]ast.ASTHandle(nil), relationSorts...),
		columns:       cloneLazyColumns(columns),
		sorts:         cloneLazySorts(sorts),
	}
	return t.replaceRoot(&lazyTableNode{kind: LazyTableFilterInterpreted, signature: Signature{domains: t.root.signature.Domains()}, left: t.root, interpreted: filter})
}

func cloneLazyColumns(source map[uint32]int) map[uint32]int {
	result := make(map[uint32]int, len(source))
	for variable, column := range source {
		result[variable] = column
	}
	return result
}

func cloneLazySorts(source map[uint32]ast.ASTHandle) map[uint32]ast.ASTHandle {
	result := make(map[uint32]ast.ASTHandle, len(source))
	for variable, sort := range source {
		result[variable] = sort
	}
	return result
}
