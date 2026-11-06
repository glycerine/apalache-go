package shell

// Source: src/shell/main.cpp:214-245, retained global-parameter output options.

import (
	"errors"
	"io"
)

var ErrShellTerminalOutputOpen = errors.New("shell: terminal output action is not yet ported")

// WriteTerminalAction executes the source's parameter display branches after
// Parse selects a terminal action. The borrowed stream receives each reached
// insertion directly, including the -pp diagnostic before descriptor lookup.
func (c *CommandLine) WriteTerminalAction(out io.Writer) error {
	if c == nil {
		return ErrShellOptionService
	}
	if c.Action == CommandContinue {
		return nil
	}
	if out == nil {
		return ErrShellOptionService
	}
	if c.Action == CommandHelp {
		return WriteUsage(out)
	}
	if c.Action == CommandVersion {
		return WriteVersion(out)
	}
	if c.Globals == nil {
		return ErrShellOptionService
	}
	switch c.Action {
	case CommandDisplayParams:
		return c.Globals.SourceDisplayTo(out, 0, false, false)
	case CommandDisplayParamDescriptions:
		return c.Globals.SourceDisplayTo(out, 0, false, true)
	case CommandDisplayModules:
		if err := c.Globals.SourceDisplayModulesTo(out); err != nil {
			return err
		}
		const suffix = "\nUse -pm:name to display all parameters available at module 'name'\n"
		n, err := io.WriteString(out, suffix)
		if err != nil {
			return err
		}
		if n != len(suffix) {
			return io.ErrShortWrite
		}
		return nil
	case CommandDisplayModule:
		return c.Globals.SourceDisplayModuleTo(out, c.ActionArgument)
	case CommandDisplayParameter:
		return c.Globals.SourceDisplayParameterTo(out, c.ActionArgument)
	default:
		return ErrShellTerminalOutputOpen
	}
}

// ParseAndWrite runs the command-line scan and the reached terminal output
// branch. The executable's exit and frontend dispatch still belong to main.
func (c *CommandLine) ParseAndWrite(args []string, out io.Writer) error {
	if err := c.Parse(args); err != nil {
		return err
	}
	return c.WriteTerminalAction(out)
}
