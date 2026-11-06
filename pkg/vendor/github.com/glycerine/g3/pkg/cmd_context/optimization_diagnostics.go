package cmd_context

// Source: src/shell/opt_frontend.cpp model_validate result diagnostics and
// src/api/api_opt.cpp optimize reason-unknown consumer.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (c *Context) OptimizationReasonUnknown() (string, error) {
	if err := c.valid(); err != nil {
		return "", err
	}
	return c.ReasonUnknown()
}

// OptimizationException returns an exception caught by the optimizer shell
// boundary. Solver unknown results without a caught exception return found=false.
func (c *Context) OptimizationException() (message string, found bool, err error) {
	if err = c.valid(); err != nil {
		return "", false, err
	}
	message = c.optimizationException
	return message, message != "", nil
}

func (c *Context) writeOptimizationModelValidation(out io.Writer, expressions []ast.ASTHandle) (violations uint32, resultErr error) {
	if err := c.valid(); err != nil {
		return 0, err
	}
	if out == nil {
		return 0, io.ErrClosedPipe
	}
	md, err := c.Model()
	if err != nil {
		return 0, err
	}
	defer func() { resultErr = errors.Join(resultErr, md.DecRef()) }()
	evaluator, err := model.NewModelEvaluator(md, c.config.ModelEvaluator)
	if evaluator != nil {
		defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	}
	if err != nil {
		return 0, err
	}
	for _, expression := range expressions {
		value, err := evaluator.EvaluateValue(expression)
		if err != nil {
			return violations, err
		}
		handle, err := c.manager.RefGet(value)
		if err != nil {
			return violations, errors.Join(err, c.manager.DelRef(&value))
		}
		truth := c.manager.IsTrue(handle)
		if !truth {
			violations++
			if err = c.WriteExpression(out, expression); err == nil {
				_, err = io.WriteString(out, " evaluates to: ")
			}
			if err == nil {
				err = c.WriteExpression(out, handle)
			}
			if err == nil {
				_, err = io.WriteString(out, "\n")
			}
		}
		err = errors.Join(err, c.manager.DelRef(&value))
		if err != nil {
			return violations, err
		}
	}
	return violations, nil
}

// WriteOptimizationModelValidation evaluates every retained hard assertion in
// the latest optimization model and prints only failures.
func (c *Context) WriteOptimizationModelValidation(out io.Writer) (uint32, error) {
	if err := c.valid(); err != nil {
		return 0, err
	}
	expressions := make([]ast.ASTHandle, len(c.assertions))
	for i := range c.assertions {
		expressions[i] = c.assertions[i].expression
	}
	return c.writeOptimizationModelValidation(out, expressions)
}

func (c *Context) OptimizationModelValid() (bool, error) {
	violations, err := c.WriteOptimizationModelValidation(io.Discard)
	return violations == 0, err
}
