package qe

// Source: src/qe/qe_bv_plugin.cpp project local model_evaluator, val_x and
// rational value ownership.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type bitVectorModelProjectCall struct {
	plugin    *BitVectorPlugin
	evaluator *model.ModelEvaluator
	value     ast.ASTRef
	numeral   numeral.OwnedRational
	width     uint32
	closed    bool
}

func (p *BitVectorPlugin) newModelProjectCall(source *model.Model, config Config) (*bitVectorModelProjectCall, error) {
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
	number, err := p.manager.NumeralService().NewValue()
	if err != nil {
		return nil, errors.Join(err, p.manager.DelRef(&value), evaluator.Close())
	}
	return &bitVectorModelProjectCall{plugin: p, evaluator: evaluator, value: value, numeral: number}, nil
}

func (c *bitVectorModelProjectCall) close() error {
	if c == nil || c.closed {
		return nil
	}
	c.closed = true
	return errors.Join(c.plugin.manager.NumeralService().CleanupValue(&c.numeral), c.plugin.manager.DelRef(&c.value), c.evaluator.Close())
}
