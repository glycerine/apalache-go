package api

import (
	"bytes"
	"errors"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/shell"
)

type apiWCNFInvocation struct {
	command  *cmd_context.Context
	frontend *shell.WCNFFrontend
	output   bytes.Buffer
}

func newAPIWCNFInvocation(options *WCNFRunOptions) (*apiWCNFInvocation, error) {
	c, e := cmd_context.NewAuxiliaryContext()
	if e != nil {
		return nil, e
	}
	r := &apiWCNFInvocation{command: c}
	o := shell.WCNFFrontendOptions{}
	if options != nil {
		o.DisplayStatistics = options.DisplayStatistics
	}
	r.frontend, e = shell.NewWCNFFrontend(c, &r.output, o)
	if e != nil {
		return nil, errors.Join(e, c.Close())
	}
	return r, nil
}
func (i *apiWCNFInvocation) close() error {
	if i == nil || i.command == nil {
		return nil
	}
	e := i.command.Close()
	i.command, i.frontend = nil, nil
	return e
}
