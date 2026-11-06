package api

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
)

func (c *SMTContext) PrepareOptimizationPipeline(zeroOne []ast.ASTHandle) (*opt.ObjectivePipeline, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.command.PrepareOptimizationPipeline(zeroOne)
}
