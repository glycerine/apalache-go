package cmd_context

// Source: src/api/api_opt.cpp:360-373, Z3_optimize_get_assertions.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// OptimizationAssertions returns independently owned hard constraints in
// scoped insertion order, matching the native owning AST vector.
func (c *Context) OptimizationAssertions() (result []ast.ASTRef, resultErr error) {
	if err := c.valid(); err != nil {
		return nil, err
	}
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr == nil {
			return
		}
		for i := len(result) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, manager.DelRef(&result[i]))
		}
		result = nil
	}()
	for i := range c.assertions {
		ref, err := manager.NewRef(c.assertions[i].expression)
		if err != nil {
			return nil, err
		}
		result = append(result, ref)
	}
	return result, nil
}
