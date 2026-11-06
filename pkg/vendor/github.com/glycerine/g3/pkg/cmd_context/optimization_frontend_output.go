package cmd_context

// Source: src/shell/opt_frontend.cpp display_results.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/opt"
)

// WriteOptimizationFrontendValues prints the optimization shell's objective
// bounds in the source shell format.
func (c *Context) WriteOptimizationFrontendValues(out io.Writer) error {
	if err := c.valid(); err != nil {
		return err
	}
	if out == nil {
		return fmt.Errorf("optimization: nil output")
	}
	for index := range c.objectives {
		lower, err := c.ObjectiveBound(uint32(index), true)
		if err != nil {
			return err
		}
		upper, err := c.ObjectiveBound(uint32(index), false)
		if err != nil {
			return err
		}
		equal := lower.Value.Cmp(&upper.Value) == 0
		prefix := "  ["
		if equal {
			prefix = "   "
		}
		if _, err = io.WriteString(out, prefix); err != nil {
			return err
		}
		if err = c.writeOptimizationFrontendBound(out, &lower); err != nil {
			return err
		}
		if !equal {
			if _, err = io.WriteString(out, ":"); err != nil {
				return err
			}
			if err = c.writeOptimizationFrontendBound(out, &upper); err != nil {
				return err
			}
			if _, err = io.WriteString(out, "]"); err != nil {
				return err
			}
		}
		if _, err = io.WriteString(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func (c *Context) writeOptimizationFrontendBound(out io.Writer, bound *OptimizationBound) error {
	ref, err := opt.ArithmeticValueExpression(c.manager, &bound.Value, bound.Integer)
	if err != nil {
		return err
	}
	h, err := c.manager.RefGet(ref)
	if err == nil {
		err = c.WriteExpression(out, h)
	}
	return errors.Join(err, c.manager.DelRef(&ref))
}
