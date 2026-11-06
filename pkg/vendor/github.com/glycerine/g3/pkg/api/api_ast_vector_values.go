package api

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *SMTContext) NewASTVectorValues(values ...ast.ASTHandle) (result *ASTVector, resultErr error) {
	vector, err := c.NewASTVector()
	if err != nil {
		return nil, err
	}
	result = vector
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, vector.Close())
			result = nil
		}
	}()
	for _, value := range values {
		if err = vector.object.push(value); err != nil {
			return nil, err
		}
	}
	return vector, nil
}

func (v *ASTVector) Values() ([]ast.ASTHandle, error) {
	if err := v.check(); err != nil {
		return nil, err
	}
	size, err := v.object.size()
	if err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, size)
	for index := range result {
		result[index], err = v.object.get(uint32(index))
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
