package api

import "errors"

func (c *SMTContext) RunOPBFile(path string, options *OPBRunOptions) (result OPBRunResult, resultErr error) {
	if err := c.open(); err != nil {
		return result, err
	}
	invocation, err := newAPIOPBInvocation(options)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, invocation.close()) }()
	if err = invocation.frontend.RunFile(path); err != nil {
		result.Output = invocation.output.String()
		return result, err
	}
	return invocation.result()
}
