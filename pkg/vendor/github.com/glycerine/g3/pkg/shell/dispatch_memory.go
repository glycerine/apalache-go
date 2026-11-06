package shell

// Source: src/shell/main.cpp:294-296,301-337; src/util/file_path.h:23-34.

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
	ufilepath "github.com/glycerine/g3/pkg/util/filepath"
)

const shellDefaultOOM = "ERROR: out of memory"
const shellSMT2OOM = "(error \"out of memory\")"

// BeginMemoryPolicy is the source main entry's first fatal-message selection,
// reached after memory initialization and before argument parsing.
func (c *CommandLine) BeginMemoryPolicy(policy *diagnostic.MemoryFatalContext) error {
	if c == nil || policy == nil {
		return ErrShellOptionService
	}
	message := shellDefaultOOM
	return policy.ExitWhenOutOfMemoryLiteral(true, &message)
}

// ResolveInputKind applies the source default and last-dot extension table.
// The caller reaches it after the main input-file and standard-input checks.
func (c *CommandLine) ResolveInputKind() (InputKind, error) {
	if c == nil || c.Input == nil {
		return InputUnspecified, ErrShellOptionService
	}
	if c.Kind != InputUnspecified {
		return c.Kind, nil
	}
	c.Kind = InputSMT2
	name, selected := c.Input.Selected()
	if !selected {
		return c.Kind, nil
	}
	extension, found := ufilepath.Extension(name)
	if !found {
		return c.Kind, nil
	}
	switch extension {
	case "datalog", "dl":
		c.Kind = InputDatalog
	case "dimacs", "cnf":
		c.Kind = InputDIMACS
	case "wcnf":
		c.Kind = InputWCNF
	case "opb":
		c.Kind = InputOPB
	case "log":
		c.Kind = InputZ3Log
	case "mps", "sif", "MPS", "SIF":
		c.Kind = InputMPS
	}
	return c.Kind, nil
}

// ConfigureMemoryPolicyForDispatch runs just before the selected frontend.
// Only the SMT-LIB2 branch replaces the initial fatal message in source main.
func (c *CommandLine) ConfigureMemoryPolicyForDispatch(policy *diagnostic.MemoryFatalContext) (InputKind, error) {
	if policy == nil {
		return InputUnspecified, ErrShellOptionService
	}
	kind, err := c.ResolveInputKind()
	if err != nil {
		return InputUnspecified, err
	}
	if kind == InputSMT2 {
		message := shellSMT2OOM
		if err := policy.ExitWhenOutOfMemoryLiteral(true, &message); err != nil {
			return kind, err
		}
	}
	return kind, nil
}
