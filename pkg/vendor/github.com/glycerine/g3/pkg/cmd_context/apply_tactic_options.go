package cmd_context

// Source: src/cmd_context/tactic_cmds.cpp:255-270, apply_tactic_cmd options.

import uparams "github.com/glycerine/g3/pkg/util/params"

type applyTacticSettings struct {
	print, printProof, printModelConverter bool
	printBenchmark, printDependencies      bool
	printStatistics                        bool
	timeout, rlimit                        uint32
}

func (c *Context) applyTacticSettings(p *uparams.ParamsRef, defaultTimeout, defaultRLimit uint32) (s applyTacticSettings, err error) {
	readBool := func(name string, fallback bool) (bool, error) {
		return c.config.Params.GetBoolName(p, &name, fallback)
	}
	if s.print, err = readBool("print", true); err != nil {
		return s, err
	}
	if s.printProof, err = readBool("print_proof", false); err != nil {
		return s, err
	}
	if s.printModelConverter, err = readBool("print_model_converter", false); err != nil {
		return s, err
	}
	if s.printBenchmark, err = readBool("print_benchmark", false); err != nil {
		return s, err
	}
	if s.printDependencies, err = readBool("print_dependencies", false); err != nil {
		return s, err
	}
	if s.printStatistics, err = readBool("print_statistics", false); err != nil {
		return s, err
	}
	timeout, rlimit := "timeout", "rlimit"
	if s.timeout, err = c.config.Params.GetUIntName(p, &timeout, defaultTimeout); err != nil {
		return s, err
	}
	s.rlimit, err = c.config.Params.GetUIntName(p, &rlimit, defaultRLimit)
	return s, err
}
