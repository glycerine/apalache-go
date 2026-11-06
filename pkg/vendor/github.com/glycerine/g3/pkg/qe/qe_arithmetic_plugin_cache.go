package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type arithmeticPluginBounds struct {
	variable  ast.ASTRef
	formula   ast.ASTRef
	processor *ArithmeticBoundsProcessor
}

func (e *arithmeticPluginBounds) close(m *ast.ASTManager) error {
	if e == nil {
		return nil
	}
	return errors.Join(e.processor.Close(), m.DelRef(&e.formula), m.DelRef(&e.variable))
}

func (p *ArithmeticPlugin) entry(variable *ast.ContainsApp, formula ast.ASTHandle) (*arithmeticPluginBounds, ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return nil, ast.ASTHandle{}, err
	}
	if variable == nil {
		return nil, ast.ASTHandle{}, ast.ErrASTHandle
	}
	target, err := variable.Target()
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	entry := p.bounds[arithmeticPluginKey{target, formula}]
	if entry == nil {
		return nil, target, ast.ErrASTDomain
	}
	return entry, target, nil
}

func (p *ArithmeticPlugin) insertEntry(target, formula ast.ASTHandle, processor *ArithmeticBoundsProcessor) (resultErr error) {
	e := &arithmeticPluginBounds{processor: processor}
	var err error
	e.variable, err = p.manager.NewRef(target)
	if err != nil {
		return err
	}
	e.formula, err = p.manager.NewRef(formula)
	if err != nil {
		return errors.Join(err, p.manager.DelRef(&e.variable))
	}
	p.bounds[arithmeticPluginKey{target, formula}] = e
	return nil
}
