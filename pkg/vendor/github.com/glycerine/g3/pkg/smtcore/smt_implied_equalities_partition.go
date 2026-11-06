package smtcore

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *smtImpliedEqualityEngine) partition() error {
	for index, term := range e.terms {
		sort, err := e.manager.GetSort(term)
		if err != nil {
			return fmt.Errorf("implied equality term %d sort: %w", index, err)
		}
		e.bySort[sort] = append(e.bySort[sort], uint32(index))
	}
	return nil
}

func (e *smtImpliedEqualityEngine) isArraySort(sort ast.ASTHandle) bool {
	family, err := e.manager.GetFamilyID(sort)
	if err != nil {
		return false
	}
	arrayFamily, err := e.manager.ArrayFamilyID()
	return err == nil && family == arrayFamily
}
