package api

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/opt"
)

func (o *APIOptimize) objectiveBound(index uint32, lower bool) (ast.ASTRef, error) {
	if err := o.open(); err != nil {
		return ast.ASTRef{}, err
	}
	bound, err := o.command.ObjectiveBound(index, lower)
	if err != nil {
		return ast.ASTRef{}, err
	}
	manager, err := o.command.Manager()
	if err != nil {
		return ast.ASTRef{}, err
	}
	return opt.ArithmeticValueExpression(manager, &bound.Value, bound.Integer)
}

func (o *APIOptimize) objectiveBoundVector(index uint32, lower bool) (result []ast.ASTRef, resultErr error) {
	if err := o.open(); err != nil {
		return nil, err
	}
	bound, err := o.command.ObjectiveBound(index, lower)
	if err != nil {
		return nil, err
	}
	manager, err := o.command.Manager()
	if err != nil {
		return nil, err
	}
	return optimizationBoundVector(manager, bound)
}

func optimizationBoundVector(manager *ast.ASTManager, bound cmd_context.OptimizationBound) (result []ast.ASTRef, resultErr error) {
	defer func() {
		if resultErr != nil {
			for i := range result {
				resultErr = errors.Join(resultErr, manager.DelRef(&result[i]))
			}
			result = nil
		}
	}()
	components := bound.Components()
	for i := range components {
		value := &components[i]
		handle, err := manager.ArithmeticNumeralRat(value, value.IsInt())
		if err != nil {
			return result, err
		}
		ref, err := manager.NewRef(handle)
		if err != nil {
			return result, err
		}
		result = append(result, ref)
	}
	return result, nil
}
