package api

import (
	"errors"
	"io"
	"strings"
)

type apiMPSInput struct {
	reader io.Reader
	name   string
}

func (c *SMTContext) RunMPS(input io.Reader, name string, options *MPSRunOptions) (result MPSRunResult, resultErr error) {
	if err := c.open(); err != nil {
		return result, err
	}
	if input == nil {
		return result, ErrSMTContextOwner
	}
	invocation, err := newAPIMPSInvocation(&apiMPSInput{reader: input, name: name}, options)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, invocation.close()) }()
	if err = invocation.frontend.Run(); err != nil {
		result.Output = invocation.output.String()
		return result, err
	}
	return invocation.result()
}

func (c *SMTContext) RunMPSString(input, name string, options *MPSRunOptions) (MPSRunResult, error) {
	return c.RunMPS(strings.NewReader(input), name, options)
}
