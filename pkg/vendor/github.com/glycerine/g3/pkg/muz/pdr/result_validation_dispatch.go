package pdr

// Source: pdr_context.cpp context::validate.

import "github.com/glycerine/g3/pkg/util/lbool"

func (c *Context) validateLatestResult() error {
	if !c.validateResult {
		return nil
	}
	switch c.lastResult {
	case lbool.True:
		return c.validateSATResult(c.queryDecl)
	case lbool.False:
		return c.validateUNSATResult()
	default:
		return nil
	}
}

func (c *Context) finishValidatedResult(status lbool.LBool) error {
	c.lastResult = status
	if err := c.validateLatestResult(); err != nil {
		c.lastResult = lbool.Undef
		return err
	}
	return nil
}
