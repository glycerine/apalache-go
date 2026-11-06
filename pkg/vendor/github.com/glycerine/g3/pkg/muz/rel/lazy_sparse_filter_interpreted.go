package rel

// Source: src/muz/rel/dl_lazy_table.cpp lazy_table_filter_interpreted and the
// filter-interpreted/project fusion in lazy_table_project::force.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

type lazySparseInterpretedFilter struct {
	owner         *lazySparseConditionOwner
	manager       *RelationManager
	rewriter      *astrewriter.TheoryRewriter
	relationSorts []ast.ASTHandle
	columns       map[uint32]int
	sorts         map[uint32]ast.ASTHandle
}

func (f *lazySparseInterpretedFilter) conditions() ([]ast.ASTHandle, error) {
	if f == nil || f.owner == nil || f.owner.manager == nil || len(f.owner.refs) == 0 || f.rewriter == nil {
		return nil, ErrLazySparse
	}
	if f.manager == nil || f.manager.open() != nil || f.rewriter.Manager() != f.owner.manager {
		return nil, ErrLazySparse
	}
	result := make([]ast.ASTHandle, len(f.owner.refs))
	for index := range f.owner.refs {
		handle, err := f.owner.manager.RefGet(f.owner.refs[index])
		if err != nil {
			return nil, err
		}
		result[index] = handle
	}
	return result, nil

}

func (f *lazySparseInterpretedFilter) condition() (ast.ASTHandle, error) {
	conditions, err := f.conditions()
	if err != nil || len(conditions) != 1 {
		return ast.ASTHandle{}, ErrLazySparse
	}
	return conditions[0], nil
}

func validateLazySparseCondition(manager *RelationManager, signature Signature, relationSorts []ast.ASTHandle, rewriter *astrewriter.TheoryRewriter, condition ast.ASTHandle) error {
	if manager == nil || manager.open() != nil || rewriter == nil || rewriter.Manager() != manager.manager {
		return ErrLazySparse
	}
	expected, err := tableRelationSignature(manager, relationSorts)
	if err != nil {
		return err
	}
	if !expected.equal(signature) {
		return ErrLazySparse
	}
	kind, err := manager.manager.GetKind(condition)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp {
		return fmt.Errorf("%w: interpreted condition must be an application", ErrLazySparse)
	}
	boolean, err := manager.manager.MkBoolSort()
	if err != nil {
		return err
	}
	sort, err := manager.manager.GetSort(condition)
	if err != nil {
		return err
	}
	if sort != boolean {
		return fmt.Errorf("%w: interpreted condition is not Boolean", ErrLazySparse)
	}
	used, err := ast.NewASTUsedVars(manager.manager)
	if err != nil {
		return err
	}
	if err = used.Collect(condition); err != nil {
		return err
	}
	if used.MaxFoundVarIndexPlusOne() > uint32(len(relationSorts)) {
		return ErrLazySparse
	}
	for variable := uint32(0); variable < used.MaxFoundVarIndexPlusOne(); variable++ {
		variableSort := used.Contains(variable)
		if variableSort != (ast.ASTHandle{}) && variableSort != relationSorts[variable] {
			return fmt.Errorf("%w: interpreted variable sort mismatch", ErrLazySparse)
		}
	}
	return nil
}

// FilterInterpreted appends a deferred source condition and owns its AST
// reference until the final node owner closes successfully.
func (t *LazySparseTable) FilterInterpreted(manager *RelationManager, relationSorts []ast.ASTHandle, rewriter *astrewriter.TheoryRewriter, condition ast.ASTHandle) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := validateLazySparseCondition(manager, t.root.signature, relationSorts, rewriter, condition); err != nil {
		return err
	}
	owner := &lazySparseConditionOwner{engine: t.engine, manager: manager.manager}
	t.engine.conditions = append(t.engine.conditions, owner)
	reference, err := manager.manager.NewRef(condition)
	if err != nil {
		_, cleanupErr := owner.release()
		return errors.Join(err, cleanupErr)
	}
	owner.refs = append(owner.refs, reference)
	payload := &lazySparseInterpretedFilter{
		owner: owner, manager: manager, rewriter: rewriter, relationSorts: append([]ast.ASTHandle(nil), relationSorts...),
	}
	if err = t.replaceLazySparseRoot(&lazySparseNode{
		kind: LazyTableFilterInterpreted, signature: Signature{domains: t.root.signature.Domains()}, functional: t.root.functional,
		left: t.root, interpreted: payload,
	}); err != nil {
		_, cleanupErr := owner.release()
		return errors.Join(err, cleanupErr)
	}
	return nil
}

func evalLazySparseInterpreted(engine *LazySparseEngine, node *lazySparseNode) (*SparseTable, error) {
	result, err := engine.transferLazySparseSource(node, &node.left)
	if err != nil {
		return nil, err
	}
	if node.interpreted.columns != nil || node.interpreted.sorts != nil {
		conditions, conditionErr := node.interpreted.conditions()
		if conditionErr != nil {
			return nil, conditionErr
		}
		err = filterSparseInterpretedBound(result, node.interpreted.manager, node.interpreted.rewriter, conditions, node.interpreted.columns, node.interpreted.sorts)
	} else {
		condition, conditionErr := node.interpreted.condition()
		if conditionErr != nil {
			return nil, conditionErr
		}
		err = result.FilterInterpreted(node.interpreted.manager, node.interpreted.relationSorts, node.interpreted.rewriter, condition)
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func evalLazySparseInterpretedProject(engine *LazySparseEngine, filter *lazySparseNode, removed []int) (*SparseTable, error) {
	// Evaluate the filter node itself: mutating force transfers the table and
	// clears its child, while retryForce keeps an incomplete result retryable.
	source, err := engine.eval(filter)
	if err != nil {
		return nil, err
	}
	if filter.interpreted.columns != nil || filter.interpreted.sorts != nil {
		conditions, conditionErr := filter.interpreted.conditions()
		if conditionErr != nil {
			return nil, conditionErr
		}
		clone, cloneErr := source.Clone()
		if cloneErr != nil {
			return nil, cloneErr
		}
		if conditionErr = filterSparseInterpretedBound(clone, filter.interpreted.manager, filter.interpreted.rewriter, conditions, filter.interpreted.columns, filter.interpreted.sorts); conditionErr != nil {
			return nil, errors.Join(conditionErr, clone.Close())
		}
		projected, projectErr := clone.Project(removed)
		closeErr := clone.Close()
		if projectErr != nil || closeErr != nil {
			if projected != nil {
				closeErr = errors.Join(closeErr, projected.Close())
			}
			return nil, errors.Join(projectErr, closeErr)
		}
		return projected, nil
	}
	condition, conditionErr := filter.interpreted.condition()
	if conditionErr != nil {
		return nil, conditionErr
	}
	return source.FilterInterpretedAndProject(filter.interpreted.manager, filter.interpreted.relationSorts, filter.interpreted.rewriter, condition, removed)
}

func evalLazySparseNullaryFilter(engine *LazySparseEngine, node *lazySparseNode) (*HashTable, error) {
	source, err := engine.evalFiniteNode(node.left)
	if err != nil {
		return nil, err
	}
	cloneValue, err := source.CloneFinite()
	if err != nil {
		return nil, err
	}
	result, ok := cloneValue.(*HashTable)
	if !ok {
		return nil, errors.Join(ErrLazySparse, cloneValue.Close())
	}
	switch node.kind {
	case LazyTableFilterIdentical:
		return result, nil
	case LazyTableFilterInterpreted:
		relation := &TableRelation{manager: node.interpreted.manager, sorts: nil, table: result}
		conditions, conditionErr := node.interpreted.conditions()
		if conditionErr != nil {
			return nil, errors.Join(conditionErr, result.Close())
		}
		columns, sorts := node.interpreted.columns, node.interpreted.sorts
		if columns == nil && sorts == nil {
			columns, sorts = map[uint32]int{}, map[uint32]ast.ASTHandle{}
		}
		conditionErr = relation.FilterInterpreted(node.interpreted.rewriter, conditions, columns, sorts)
		if conditionErr != nil {
			return nil, errors.Join(conditionErr, result.Close())
		}
		return result, nil
	case LazyTableFilterByNegation:
		negative, negativeErr := engine.evalFiniteNode(node.right)
		if negativeErr != nil {
			return nil, errors.Join(negativeErr, result.Close())
		}
		negativeTable, ok := negative.(*HashTable)
		if !ok {
			return nil, errors.Join(ErrLazySparse, result.Close())
		}
		if negativeErr = result.FilterByNegation(negativeTable, node.columns1, node.columns2); negativeErr != nil {
			return nil, errors.Join(negativeErr, result.Close())
		}
		return result, nil
	default:
		return nil, errors.Join(ErrLazySparse, result.Close())
	}
}
