package api

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
)

func (c *SMTContext) PreparePurifiedOptimizationPipeline(zeroOne []ast.ASTHandle) (*opt.PurifiedObjectivePipeline, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.command.PreparePurifiedOptimizationPipeline(zeroOne)
}
