package api

// Source: src/api/api_opt.cpp optimize reason-unknown and retained model
// diagnostic consumers.

import "io"

func (c *SMTContext) OptimizationReasonUnknown() (string, error) {
	if err := c.open(); err != nil {
		return "", err
	}
	return c.command.OptimizationReasonUnknown()
}

func (c *SMTContext) WriteOptimizationModelValidation(out io.Writer) (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	return c.command.WriteOptimizationModelValidation(out)
}

func (c *SMTContext) OptimizationModelValid() (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	return c.command.OptimizationModelValid()
}
