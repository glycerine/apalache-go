package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (p *ArithmeticPlugin) projectCompleteStep(variable ast.ASTHandle, source *model.Model, formula ast.ASTRef, config Config, result *ArithmeticCompleteProjectionResult) (resultErr error) {
	contains, e := ast.NewContainsApp(p.manager, variable)
	if e != nil {
		return e
	}
	defer func() { resultErr = errors.Join(resultErr, contains.Close()) }()
	definition, e := p.manager.NewRef(ast.ASTHandle{})
	if e != nil {
		return e
	}
	owned := true
	defer func() {
		if owned {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&definition))
		}
	}()
	ok, e := p.ProjectModelFormulaNNFComplete(contains, source, formula, &definition, config)
	if e != nil {
		return e
	}
	if !ok {
		return ast.ErrASTDomain
	}
	if e = result.append(variable, definition); e != nil {
		return e
	}
	owned = false
	return nil
}
