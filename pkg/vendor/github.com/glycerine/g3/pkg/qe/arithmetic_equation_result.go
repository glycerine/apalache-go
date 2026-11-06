package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *ArithmeticPlugin) equationElimination(index int, formula, definition ast.ASTHandle, fresh []ast.ASTRef) (result *ArithmeticEquationElimination, resultErr error) {
	e := &ArithmeticEquationElimination{manager: p.manager, VariableIndex: uint32(index)}
	e.self = e
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, e.Close())
			result = nil
		}
	}()
	var err error
	e.Formula, err = p.manager.NewRef(formula)
	if err != nil {
		return nil, err
	}
	e.Definition, err = p.manager.NewRef(definition)
	if err != nil {
		return nil, err
	}
	e.Fresh = fresh
	return e, nil
}

func (p *ArithmeticPlugin) rewriteEquation(index int, variable, definition, formula ast.ASTHandle, fresh []ast.ASTRef) (*ArithmeticEquationElimination, error) {
	result, err := p.manager.NewRef(formula)
	if err != nil {
		return nil, err
	}
	if err = p.replacer.ApplySubstitution(variable, definition, result); err != nil {
		return nil, errors.Join(err, p.manager.DelRef(&result))
	}
	handle, err := p.manager.RefGet(result)
	if err != nil {
		return nil, errors.Join(err, p.manager.DelRef(&result))
	}
	elimination, err := p.equationElimination(index, handle, definition, fresh)
	closeErr := p.manager.DelRef(&result)
	if err != nil || closeErr != nil {
		if elimination != nil {
			// Fresh ownership returns to the caller whenever publication fails.
			elimination.Fresh = nil
			closeErr = errors.Join(closeErr, elimination.Close())
			elimination = nil
		}
		return nil, errors.Join(err, closeErr)
	}
	return elimination, nil
}
