package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

// ProjectModel retains arith_plugin::project after bounds preparation. It
// selects a source branch from the model, materializes it, and substitutes the
// cached result into formula.
func (p *ArithmeticPlugin) ProjectModel(variable *ast.ContainsApp, source *model.Model, formula ast.ASTRef, config Config) (projected bool, resultErr error) {
	if err := p.open(); err != nil {
		return false, err
	}
	root, err := p.manager.RefGet(formula)
	if err != nil {
		return false, err
	}
	entry, target, err := p.entry(variable, root)
	if err != nil {
		return false, err
	}
	call, err := p.newModelProjectCall(source, config)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, call.close()) }()
	integer, err := p.util.isInteger(target)
	if err != nil {
		return false, err
	}
	var branch uint32
	if integer {
		branch, err = call.integerBranch(entry, target)
	} else {
		branch, err = call.realBranch(entry, target)
	}
	if err != nil {
		return false, err
	}
	if err = call.apply(variable, formula, branch); err != nil {
		return false, err
	}
	return true, nil
}
