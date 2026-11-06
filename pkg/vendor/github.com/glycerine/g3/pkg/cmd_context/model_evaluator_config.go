package cmd_context

// Source: src/cmd_context/cmd_context.h model-evaluator service ownership.

import "github.com/glycerine/g3/pkg/model"

// ModelEvaluatorConfig returns a borrowed copy of the concrete evaluator
// services owned by this command context. Its members remain valid only while
// the context is open.
func (c *Context) ModelEvaluatorConfig() (model.ModelEvaluatorConfig, error) {
	if err := c.valid(); err != nil {
		return model.ModelEvaluatorConfig{}, err
	}
	return c.config.ModelEvaluator, nil
}
