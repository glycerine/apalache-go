package base

// Source: src/muz/base/dl_context.h get_num_assertions/get_assertion retained
// owning consumer surface.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *DatalogContext) BackgroundAssertions() (_ []ast.ASTRef, resultErr error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	result := make([]ast.ASTRef, 0, len(c.background))
	defer func() {
		if resultErr == nil {
			return
		}
		for i := len(result) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&result[i]))
		}
	}()
	for _, reference := range c.background {
		copy, err := c.manager.CopyRef(reference)
		if err != nil {
			return nil, err
		}
		result = append(result, copy)
	}
	return result, nil
}
