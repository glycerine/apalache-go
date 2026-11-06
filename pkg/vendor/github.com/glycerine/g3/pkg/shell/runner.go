package shell

// Source: src/shell/main.cpp:291-367 frontend composition and dispatch.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/params"
)

// RunnerConfig contains the process-independent transports of one shell
// invocation. Args excludes argv[0]. Context is borrowed when supplied.
type RunnerConfig struct {
	Args        []string
	Input       io.Reader
	Output      io.Writer
	Diagnostics io.Writer
	Context     *cmd_context.Context
}

type Runner struct {
	args         []string
	input        io.Reader
	output       io.Writer
	diagnostics  io.Writer
	context      *cmd_context.Context
	memoryPolicy *diagnostic.MemoryFatalContext
	command      *CommandLine
	inputClose   io.Closer
	owned        bool
	running      bool
	ran          bool
	closed       bool
	result       RunnerResult
}

func NewRunner(c RunnerConfig) (runner *Runner, err error) {
	if c.Input == nil || c.Output == nil {
		return nil, ErrShellOptionService
	}
	runner = &Runner{args: append([]string(nil), c.Args...), input: c.Input, output: c.Output, diagnostics: c.Diagnostics, context: c.Context}
	if runner.context == nil {
		runner.context, runner.memoryPolicy, err = cmd_context.NewStandaloneShellContext()
		if err != nil {
			return nil, err
		}
		runner.owned = true
	}
	globals, err := runner.context.GlobalParameters()
	if err != nil {
		runner.Close()
		return nil, err
	}
	runner.command = NewCommandLine(nil, nil, globals)
	return runner, nil
}

func (r *Runner) Run() (result error) {
	if r == nil || r.closed || r.running || r.ran || r.command == nil {
		return ErrShellOptionService
	}
	r.running = true
	defer func() {
		r.running = false
		r.ran = true
		r.result.Completed = true
		r.result.Error = result
	}()
	if r.memoryPolicy != nil {
		if err := r.command.BeginMemoryPolicy(r.memoryPolicy); err != nil {
			return err
		}
	}
	if err := r.command.Parse(r.args); err != nil {
		return err
	}
	r.result.Action = r.command.Action
	if r.command.Action != CommandContinue {
		return r.command.WriteTerminalAction(r.output)
	}
	globals, err := r.context.GlobalParameters()
	if err != nil {
		return err
	}
	if err = params.UpdateProcessEnvParams(globals.Current); err != nil {
		return err
	}
	_, hasFile := r.command.Input.Selected()
	if hasFile && r.command.StandardInput {
		return fmt.Errorf("using standard input to read formula")
	}
	if !hasFile && !r.command.StandardInput {
		return fmt.Errorf("input file was not specified")
	}
	var kind InputKind
	if r.memoryPolicy != nil {
		kind, err = r.command.ConfigureMemoryPolicyForDispatch(r.memoryPolicy)
	} else {
		kind, err = r.command.ResolveInputKind()
	}
	if err != nil {
		return err
	}
	r.result.Kind = kind
	if err := r.selectInput(); err != nil {
		return err
	}
	if err = r.context.RefreshGlobalParameters(); err != nil {
		return err
	}
	return r.dispatch(kind)
}
