package qe

// Source: src/qe/qe_bool_plugin.cpp bool_plugin::project.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (p *BoolPlugin) Project(variable *ast.ContainsApp, source *model.Model, formula ast.ASTRef, config Config) (resultErr error) {
	if err := p.open(); err != nil {
		return err
	}
	if source == nil {
		return ast.ErrASTHandle
	}
	current, err := p.manager.RefGet(formula)
	if err != nil {
		return err
	}
	if err = p.checkInputs(variable, current); err != nil {
		return err
	}
	target, err := variable.Target()
	if err != nil {
		return err
	}
	value, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&value)) }()
	services := model.ModelEvaluatorConfig{
		Context: config.Context, Globals: config.Globals,
		Memory: p.manager.NumeralService().RuntimeOwner(), Cooperation: config.Cooperation, Trace: config.Trace,
	}
	if err = source.Evaluate(target, value, false, services); err != nil {
		return err
	}
	evaluated, err := p.manager.RefGet(value)
	if err != nil {
		return err
	}
	if err := p.traceBoolProjection(config, target, evaluated); err != nil {
		return err
	}
	replacement, err := p.manager.MkBoolVal(p.manager.IsTrue(evaluated))
	if err != nil {
		return err
	}
	return p.replacer.ApplySubstitution(target, replacement, formula)
}
