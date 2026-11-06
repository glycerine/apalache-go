package api

// Source: src/shell/opt_frontend.cpp OPB dispatch and retained Go
// shell.OPBFrontend lifecycle.

import (
	"bytes"
	"errors"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/shell"
)

type apiOPBInvocation struct {
	command  *cmd_context.Context
	frontend *shell.OPBFrontend
	output   bytes.Buffer
}

func newAPIOPBInvocation(options *OPBRunOptions) (*apiOPBInvocation, error) {
	command, err := cmd_context.NewAuxiliaryContext()
	if err != nil {
		return nil, err
	}
	result := &apiOPBInvocation{command: command}
	shellOptions := shell.OPBFrontendOptions{}
	if options != nil {
		shellOptions.DisplayStatistics = options.DisplayStatistics
	}
	result.frontend, err = shell.NewOPBFrontend(command, &result.output, shellOptions)
	if err != nil {
		return nil, errors.Join(err, command.Close())
	}
	return result, nil
}

func (i *apiOPBInvocation) close() error {
	if i == nil || i.command == nil {
		return nil
	}
	err := i.command.Close()
	i.command, i.frontend = nil, nil
	return err
}
