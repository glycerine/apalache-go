package api

import (
	"errors"
	"io"
	"strings"
)

func (c *SMTContext) RunOPB(input io.Reader, options *OPBRunOptions) (result OPBRunResult, resultErr error) {
	if err := c.open(); err != nil {
		return result, err
	}
	if input == nil {
		return result, ErrSMTContextOwner
	}
	invocation, err := newAPIOPBInvocation(options)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, invocation.close()) }()
	if err = invocation.frontend.Run(input); err != nil {
		result.Output = invocation.output.String()
		return result, err
	}
	return invocation.result()
}

func (c *SMTContext) RunOPBString(input string, options *OPBRunOptions) (OPBRunResult, error) {
	return c.RunOPB(strings.NewReader(input), options)
}
