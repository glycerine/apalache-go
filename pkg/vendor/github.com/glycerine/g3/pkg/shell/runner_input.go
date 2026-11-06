package shell

// Source: src/shell/main.cpp:275-300 input validation and file opening.

import (
	"fmt"
	"os"
)

func (r *Runner) selectInput() error {
	name, selected := r.command.Input.Selected()
	if selected && r.command.StandardInput {
		return fmt.Errorf("using standard input to read formula")
	}
	if !selected && !r.command.StandardInput {
		return fmt.Errorf("input file was not specified")
	}
	if !selected {
		r.result.InputName = "stdin"
		return nil
	}
	r.result.InputName = name
	if r.result.Kind == InputSMT2 || r.result.Kind == InputOPB || r.result.Kind == InputWCNF {
		return nil
	}
	file, err := os.Open(name)
	if err != nil {
		return fmt.Errorf("failed to open file '%s': %w", name, err)
	}
	r.input = file
	r.inputClose = file
	return nil
}
