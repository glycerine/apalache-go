package cmd_context

// Source: src/cmd_context/basic_cmds.cpp get_option_cmd.

import (
	"errors"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func (c *Context) GetOption(name string) (string, bool, error) {
	if err := c.valid(); err != nil {
		return "", false, err
	}
	name = strings.ReplaceAll(strings.TrimPrefix(name, ":"), "-", "_")
	state, err := c.parameters.State()
	if err != nil {
		return "", false, err
	}
	boolean := func(value bool) (string, bool, error) {
		return strconv.FormatBool(value), true, nil
	}
	switch name {
	case "print_success":
		return boolean(c.PrintSuccess)
	case "interactive_mode", "produce_assertions":
		return boolean(c.InteractiveMode)
	case "produce_proofs", "produce_interpolants":
		return boolean(state.Proof)
	case "produce_unsat_cores":
		return boolean(state.UnsatCore)
	case "produce_models":
		return boolean(state.Model)
	case "produce_assignments":
		return boolean(c.ProduceAssignments)
	case "global_decls", "global_declarations":
		return boolean(c.GlobalDeclarations)
	case "random_seed":
		return strconv.FormatUint(uint64(c.RandomSeed), 10), true, nil
	case "verbosity":
		value, e := diagnostic.ProcessVerbosityLevel()
		return strconv.FormatUint(uint64(value), 10), e == nil, e
	case "regular_output_channel":
		return "stdout", true, nil
	case "diagnostic_output_channel":
		return "stderr", true, nil
	case "error_behavior":
		if c.ExitOnError {
			return "immediate-exit", true, nil
		}
		return "continued-execution", true, nil
	case "int_real_coercions":
		manager, e := c.Manager()
		if e != nil {
			return "", false, e
		}
		value, e := manager.IntRealCoercions()
		if e != nil {
			return "", false, e
		}
		return boolean(value)
	case "timeout", "rlimit":
		defaultValue := ^uint32(0)
		if name == "rlimit" {
			defaultValue = 0
		}
		value, e := c.config.Params.GetUIntName(&c.params, &name, defaultValue)
		return strconv.FormatUint(uint64(value), 10), e == nil, e
	case "expand_definitions", "produce_unsat_assumptions", "numeral_as_real", "reproducible_resource_limit":
		return "", false, nil
	}
	value, err := c.config.Globals.SourceGetValueName(name)
	if err == nil {
		return value, true, nil
	}
	var unsupported z3exception.Z3Exception
	if errors.As(err, &unsupported) {
		return "", false, nil
	}
	return "", false, err
}
