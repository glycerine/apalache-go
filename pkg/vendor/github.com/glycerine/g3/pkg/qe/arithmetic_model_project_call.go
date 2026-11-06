package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

type arithmeticModelProjectCall struct {
	plugin    *ArithmeticPlugin
	evaluator *model.ModelEvaluator
	value     ast.ASTRef
}

func (p *ArithmeticPlugin) newModelProjectCall(source *model.Model, config Config) (*arithmeticModelProjectCall, error) {
	if source == nil {
		return nil, ast.ErrASTHandle
	}
	evaluator, err := model.NewModelEvaluator(source.Core(), model.ModelEvaluatorConfig{
		Context: config.Context, Globals: config.Globals, Memory: p.manager.NumeralService().RuntimeOwner(),
		Cooperation: config.Cooperation, Trace: config.Trace,
	})
	if err != nil {
		return nil, err
	}
	value, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, errors.Join(err, evaluator.Close())
	}
	return &arithmeticModelProjectCall{plugin: p, evaluator: evaluator, value: value}, nil
}

func (c *arithmeticModelProjectCall) close() error {
	return errors.Join(c.plugin.manager.DelRef(&c.value), c.evaluator.Close())
}
