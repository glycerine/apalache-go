package api

// Source: api::context_params::m_timeout consumed by scoped API operations.

import (
	"strconv"
)

func (c *SMTContext) contextTimeout() (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	text, found, err := c.command.GetOption("timeout")
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, ErrSMTContextOwner
	}
	value, err := strconv.ParseUint(text, 10, 32)
	return uint32(value), err
}
