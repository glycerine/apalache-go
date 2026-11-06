package api

import "errors"

func (c *SMTContext) RunWCNFFile(path string, options *WCNFRunOptions) (r WCNFRunResult, resultErr error) {
	if e := c.open(); e != nil {
		return r, e
	}
	i, e := newAPIWCNFInvocation(options)
	if e != nil {
		return r, e
	}
	defer func() { resultErr = errors.Join(resultErr, i.close()) }()
	if e = i.frontend.RunFile(path); e != nil {
		r.Output = i.output.String()
		return r, e
	}
	return i.result()
}
