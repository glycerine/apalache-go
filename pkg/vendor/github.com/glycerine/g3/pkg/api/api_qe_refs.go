package api

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func apiQEOwnRefs(manager *ast.ASTManager, values []ast.ASTHandle) (result []ast.ASTRef, resultErr error) {
	result = make([]ast.ASTRef, 0, len(values))
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, apiQECloseRefs(manager, &result))
		}
	}()
	for _, value := range values {
		ref, err := manager.NewRef(value)
		if err != nil {
			return nil, err
		}
		result = append(result, ref)
	}
	return result, nil
}

func apiQECloseRefs(manager *ast.ASTManager, values *[]ast.ASTRef) (result error) {
	for index := len(*values) - 1; index >= 0; index-- {
		result = errors.Join(result, manager.DelRef(&(*values)[index]))
	}
	*values = nil
	return result
}

func apiQEApplications(manager *ast.ASTManager, values []ast.ASTHandle) error {
	for _, value := range values {
		kind, err := manager.GetKind(value)
		if err != nil {
			return err
		}
		if kind != ast.ASTApp {
			return ast.ErrASTDomain
		}
	}
	return nil
}
