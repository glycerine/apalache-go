package cmd_context

// Source: src/opt/opt_context.h num_objectives and
// src/api/api_opt.cpp:379-393.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *Context) OptimizationObjectiveCount() (uint32, error) {
	if err := c.valid(); err != nil {
		return 0, err
	}
	return uint32(len(c.objectives)), nil
}

// OptimizationObjectives returns independently owned source display terms in
// scoped insertion order. Maximization terms are negated and MaxSMT groups are
// sums of conditional penalties, matching opt::context::get_objective.
func (c *Context) OptimizationObjectives() (result []ast.ASTRef, resultErr error) {
	if err := c.valid(); err != nil {
		return nil, err
	}
	if _, err := c.Manager(); err != nil {
		return nil, err
	}
	defer func() {
		if resultErr == nil {
			return
		}
		for i := len(result) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&result[i]))
		}
		result = nil
	}()
	for i := range c.objectives {
		objective := &c.objectives[i]
		var ref ast.ASTRef
		var err error
		if objective.soft {
			ref, err = c.optimizationSoftExpression(objective)
		} else {
			ref, err = c.optimizationTermExpression(objective)
		}
		if err != nil {
			return nil, err
		}
		result = append(result, ref)
	}
	return result, nil
}
