package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func importFiniteFactsToExplanationUDoc(manager *RelationManager, sorts []ast.ASTHandle, source FiniteTable, target *UDocTableProduct) (resultErr error) {
	if manager == nil || source == nil || target == nil {
		return ErrUDocTableProduct
	}
	facts, err := source.Facts()
	if err != nil {
		return err
	}
	for _, fact := range facts {
		values, err := manager.DecodeFact(sorts, fact)
		if err != nil {
			return errors.Join(err, finishDecodedFact(manager.manager, values, nil))
		}
		handles := make([]ast.ASTHandle, len(values))
		for i, value := range values {
			handles[i], err = manager.manager.RefGet(value)
			if err != nil {
				break
			}
		}
		if err == nil {
			err = target.AddFact(handles)
		}
		err = errors.Join(err, finishDecodedFact(manager.manager, values, nil))
		if err != nil {
			return err
		}
	}
	return nil
}
