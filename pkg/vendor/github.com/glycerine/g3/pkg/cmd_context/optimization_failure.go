package cmd_context

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func (c *Context) handleOptimizationFailure(cause error) (lbool.LBool, error) {
	var fatal z3exception.Z3Error
	if errors.As(cause, &fatal) {
		return lbool.Undef, cause
	}
	canceled, err := c.manager.Canceled()
	if err != nil {
		return lbool.Undef, errors.Join(cause, err)
	}
	if canceled {
		return c.publishOptimizationUnknown(c.manager.ResourceLimit().CancelMessage())
	}
	var source z3exception.Z3Exception
	if errors.As(cause, &source) {
		c.optimizationException = source.Message()
		return c.publishOptimizationUnknown(source.Message())
	}
	return lbool.Undef, cause
}
