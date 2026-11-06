package api

// Source: src/api/api_opt.cpp:32-68.

import "github.com/glycerine/g3/pkg/cmd_context"

type APIOptimize struct {
	owner   *smtContextState
	command *cmd_context.Context
	closed  bool
}

func (c *SMTContext) NewOptimize() (*APIOptimize, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	command, err := c.command.NewParserContext()
	if err != nil {
		return nil, err
	}
	return &APIOptimize{owner: c.smtContextState, command: command}, nil
}

func (o *APIOptimize) open() error {
	if o == nil || o.owner == nil || o.command == nil || o.closed || o.owner.closed {
		return ErrSMTContextOwner
	}
	return nil
}

func (o *APIOptimize) Close() error {
	if o == nil || o.closed {
		return nil
	}
	if o.command == nil {
		return ErrSMTContextOwner
	}
	if err := o.command.Close(); err != nil {
		return err
	}
	o.closed = true
	o.owner, o.command = nil, nil
	return nil
}
