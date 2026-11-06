package cmd_context

import "github.com/glycerine/g3/pkg/model"

// FixedpointExpressionModel returns a caller-owned model for the latest
// supported expression query. The caller releases it with Close.
func (c *Context) FixedpointExpressionModel() (*model.Model, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return nil, err
	}
	return engine.ExpressionModel()
}
