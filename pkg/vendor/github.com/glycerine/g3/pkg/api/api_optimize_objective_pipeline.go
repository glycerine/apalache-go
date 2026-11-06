package api

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
)

// EncodeOptimizationFormulas exposes the retained opt_context::to_fmls
// boundary for an optimize object.
func (o *APIOptimize) EncodeOptimizationFormulas() (*opt.ObjectiveFormulaSet, error) {
	if err := o.open(); err != nil {
		return nil, err
	}
	return o.command.EncodeOptimizationFormulas()
}

func (o *APIOptimize) PrepareOptimizationPipeline(zeroOne []ast.ASTHandle) (*opt.ObjectivePipeline, error) {
	if err := o.open(); err != nil {
		return nil, err
	}
	return o.command.PrepareOptimizationPipeline(zeroOne)
}

func (o *APIOptimize) PreparePurifiedOptimizationPipeline(zeroOne []ast.ASTHandle) (*opt.PurifiedObjectivePipeline, error) {
	if err := o.open(); err != nil {
		return nil, err
	}
	return o.command.PreparePurifiedOptimizationPipeline(zeroOne)
}
