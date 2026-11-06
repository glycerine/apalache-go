package spacer

// Source: src/muz/spacer/spacer_context.cpp context::solve wraps every false
// solve_core result with frame simplification and optional result validation.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func (c *Context) finishEarlyUnreachable(inductive uint32) (lbool.LBool, error) {
	c.solved = true
	c.inductive = inductive
	c.lastResult = lbool.False
	if err := c.simplifyFormulas(); err != nil {
		return c.lastResult, err
	}
	return c.lastResult, c.validate()
}

// Source context::solve catches unknown_exception from exhausted weak
// abstraction and publishes an ordinary unknown result.
func (c *Context) finishSearchFailure(searchErr error) (lbool.LBool, error) {
	if !errors.Is(searchErr, ErrWeakAbstractionExhausted) {
		return lbool.Undef, searchErr
	}
	c.lastResult = lbool.Undef
	c.bounded = false
	return c.lastResult, nil
}
