package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (p *ArithmeticPlugin) newCompleteProject(variable *ast.ContainsApp, source *model.Model, formula ast.ASTRef, config Config) (c *arithmeticCompleteProject, resultErr error) {
	root, e := p.manager.RefGet(formula)
	if e != nil {
		return nil, e
	}
	c = &arithmeticCompleteProject{plugin: p, variable: variable, root: root}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, c.close())
			c = nil
		}
	}()
	c.atoms, e = CollectArithmeticAtomsNNF(p.manager, root)
	if e != nil {
		return nil, e
	}
	ok, e := p.PrepareAtoms(variable, root, c.atoms)
	if e != nil {
		return nil, e
	}
	if !ok {
		return nil, ast.ErrASTDomain
	}
	c.entry, c.target, e = p.entry(variable, root)
	if e != nil {
		return nil, e
	}
	c.call, e = p.newModelProjectCall(source, config)
	if e != nil {
		return nil, e
	}
	return c, nil
}
