package qe

// Source: src/qe/qe_bv_plugin.cpp project.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

// ProjectModel evaluates the eliminated variable in source and substitutes
// that concrete bit-vector value into formula.
func (p *BitVectorPlugin) ProjectModel(variable *ast.ContainsApp, source *model.Model, formula ast.ASTRef, config Config) (projected bool, resultErr error) {
	if err := p.open(); err != nil {
		return false, err
	}
	root, err := p.manager.RefGet(formula)
	if err != nil {
		return false, err
	}
	if err = p.checkInputs(variable, root); err != nil {
		return false, err
	}
	call, err := p.newModelProjectCall(source, config)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, call.close()) }()
	target, err := variable.Target()
	if err != nil {
		return false, err
	}
	if err = call.evaluate(target); err != nil {
		return false, err
	}
	if err = p.Substitute(variable, &call.numeral, formula, nil); err != nil {
		return false, err
	}
	return true, nil
}
