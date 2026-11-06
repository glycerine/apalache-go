package rel

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

// Source: src/muz/rel/dl_table_relation.cpp
// table_relation_plugin::mk_filter_interpreted_fn and
// mk_filter_interpreted_and_project_fn.

func (r *TableRelation) validateInterpretedFilter(rewriter *astrewriter.TheoryRewriter, conditions []ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle) error {
	if err := r.open(); err != nil {
		return err
	}
	if rewriter == nil || len(conditions) == 0 {
		return ErrTableRelation
	}
	boolean, err := r.manager.manager.MkBoolSort()
	if err != nil {
		return err
	}
	for _, condition := range conditions {
		sort, sortErr := r.manager.manager.GetSort(condition)
		if sortErr != nil {
			return sortErr
		}
		if sort != boolean {
			return fmt.Errorf("%w: interpreted condition is not Boolean", ErrTableRelation)
		}
	}
	for variable, sort := range sorts {
		column, found := columns[variable]
		if !found || column < 0 || column >= len(r.sorts) || r.sorts[column] != sort {
			return fmt.Errorf("%w: invalid interpreted variable binding", ErrTableRelation)
		}
	}
	return nil
}
