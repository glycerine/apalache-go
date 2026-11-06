package api

import "github.com/glycerine/g3/pkg/opt"

func (c *SMTContext) EncodeOptimizationFormulas() (*opt.ObjectiveFormulaSet, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.command.EncodeOptimizationFormulas()
}
