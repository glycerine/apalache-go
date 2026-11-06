package shell

// Source: src/shell/main.cpp:301-343 retained frontend switch.

import (
	"errors"
	"fmt"
)

func (r *Runner) dispatch(kind InputKind) (result error) {
	globals, err := r.context.GlobalParameters()
	if err != nil {
		return err
	}
	switch kind {
	case InputDatalog:
		return RunDatalogFrontend(r.context, r.result.InputName, r.output)
	case InputSMT2:
		var frontend *SMTLIBFrontend
		if _, selected := r.command.Input.Selected(); selected {
			frontend, err = OpenSMTLIBFrontend(r.result.InputName, SMTLIBFileFrontendConfig{
				Context: r.context, Output: r.output, Diagnostics: r.diagnostics,
				DisplayStatistics: r.command.DisplayStatistics, Timer: r.command.Timer,
			})
		} else {
			frontend, err = NewSMTLIBFrontend(SMTLIBFrontendConfig{
				Context: r.context, Input: r.input, Output: r.output, Diagnostics: r.diagnostics,
				DisplayStatistics: r.command.DisplayStatistics, Timer: r.command.Timer,
			})
		}
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, frontend.Close()) }()
		return frontend.Run()
	case InputDIMACS:
		frontend, err := NewDIMACSFrontend(DIMACSFrontendConfig{
			Globals: globals, Timer: r.command.Timer, Input: r.input, Writer: r.output,
			DisplayStatistics: r.command.DisplayStatistics,
		})
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, frontend.Close()) }()
		return frontend.Run()
	case InputWCNF:
		frontend, err := NewWCNFFrontend(r.context, r.output, WCNFFrontendOptions{DisplayStatistics: r.command.DisplayStatistics, Timer: r.command.Timer, Diagnostics: r.diagnostics})
		if err != nil {
			return err
		}
		if _, selected := r.command.Input.Selected(); selected {
			return frontend.RunFile(r.result.InputName)
		}
		return frontend.Run(r.input)
	case InputOPB:
		frontend, err := NewOPBFrontend(r.context, r.output, OPBFrontendOptions{DisplayStatistics: r.command.DisplayStatistics, Timer: r.command.Timer, Diagnostics: r.diagnostics})
		if err != nil {
			return err
		}
		if _, selected := r.command.Input.Selected(); selected {
			return frontend.RunFile(r.result.InputName)
		}
		return frontend.Run(r.input)
	case InputMPS:
		frontend, err := NewLPFrontend(LPFrontendConfig{
			Globals: globals, Input: r.input, Output: r.output, Name: r.result.InputName,
		})
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, frontend.Close()) }()
		return frontend.Run()
	default:
		return fmt.Errorf("input frontend %s is not ported", InputKindName(kind))
	}
}

func InputKindName(kind InputKind) string {
	switch kind {
	case InputSMT2:
		return "SMT-LIB2"
	case InputDatalog:
		return "Datalog"
	case InputDIMACS:
		return "DIMACS"
	case InputWCNF:
		return "WCNF"
	case InputOPB:
		return "OPB"
	case InputZ3Log:
		return "Z3 log"
	case InputMPS:
		return "MPS"
	default:
		return "unspecified"
	}
}
