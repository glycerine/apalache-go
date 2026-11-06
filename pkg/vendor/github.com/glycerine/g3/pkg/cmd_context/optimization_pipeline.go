package cmd_context

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
)

// PrepareOptimizationPipeline retains opt_context::to_fmls/from_fmls and
// internalize as one owned Go preparation step.
func (c *Context) PrepareOptimizationPipeline(zeroOne []ast.ASTHandle) (pipeline *opt.ObjectivePipeline, resultErr error) {
	set, err := c.EncodeOptimizationFormulas()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, set.Close()) }()
	decoded, err := set.Decode()
	if err != nil {
		return nil, err
	}
	decomposer, err := opt.NewObjectiveDecomposer(c.manager, zeroOne)
	if err != nil {
		return nil, err
	}
	decomposed, _, err := decomposer.Decompose(decoded)
	if err != nil {
		return nil, errors.Join(err, decomposer.Close())
	}
	pipeline, err = opt.NewObjectivePipeline(c.manager, &c.params, c.logic, c.config, decoded.Hard, decomposed)
	return pipeline, errors.Join(err, decomposer.Close())
}
