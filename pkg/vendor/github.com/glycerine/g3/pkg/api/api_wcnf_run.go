package api

import (
	"errors"
	"io"
	"strings"
)

func (c *SMTContext) RunWCNF(input io.Reader, options *WCNFRunOptions) (r WCNFRunResult, resultErr error) {
	if e := c.open(); e != nil {
		return r, e
	}
	if input == nil {
		return r, ErrSMTContextOwner
	}
	i, e := newAPIWCNFInvocation(options)
	if e != nil {
		return r, e
	}
	defer func() { resultErr = errors.Join(resultErr, i.close()) }()
	if e = i.frontend.Run(input); e != nil {
		r.Output = i.output.String()
		return r, e
	}
	return i.result()
}
func (c *SMTContext) RunWCNFString(input string, options *WCNFRunOptions) (WCNFRunResult, error) {
	return c.RunWCNF(strings.NewReader(input), options)
}
