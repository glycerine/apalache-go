package shell

// Source: src/shell/main.cpp:119-136,204-206,275-286, concrete input-file branches.

import (
	"strings"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/text"
)

// InputFileOption owns the selected command-line filename. The source uses a
// non-null pointer to distinguish an empty filename from no filename.
type InputFileOption struct {
	filename string
	selected bool
	warnings *diagnostic.WarningService
}

func NewInputFileOption(warnings *diagnostic.WarningService) *InputFileOption {
	if warnings == nil {
		warnings = diagnostic.BorrowProcessWarningService()
	}
	return &InputFileOption{warnings: warnings}
}

func (o *InputFileOption) Selected() (string, bool) {
	if o == nil {
		return "", false
	}
	return o.filename, o.selected
}

// ApplyFileOption implements -file[:name]. Unlike positional input, source
// g_input_file = opt_arg replaces any previous value; absent colon clears the
// pointer, while a present empty argument selects an empty filename.
func (o *InputFileOption) ApplyFileOption(argument *string) error {
	if o == nil || o.warnings == nil {
		return arena.ErrInvalidSlot
	}
	if argument == nil {
		o.filename = ""
		o.selected = false
		return nil
	}
	o.filename = (*argument)[:text.CStringLength(*argument)]
	o.selected = true
	return nil
}

// AcceptPositional implements the ordinary filename branch. A duplicate
// leaves the first filename in place, even when warning delivery fails.
func (o *InputFileOption) AcceptPositional(argument string) error {
	if o == nil || o.warnings == nil {
		return arena.ErrInvalidSlot
	}
	if o.selected {
		return o.warnings.Warning("input file was already specified.")
	}
	o.filename = argument[:text.CStringLength(argument)]
	o.selected = true
	return nil
}

// AcceptAfterDoubleDash implements the terminal -- branch. The source first
// joins every remaining argv string with spaces, then checks for a previous
// filename. An empty remainder still selects a non-null empty filename.
func (o *InputFileOption) AcceptAfterDoubleDash(arguments []string) error {
	if o == nil || o.warnings == nil {
		return arena.ErrInvalidSlot
	}
	var joined strings.Builder
	for i, argument := range arguments {
		if i != 0 {
			joined.WriteByte(' ')
		}
		joined.WriteString(argument[:text.CStringLength(argument)])
	}
	if o.selected {
		return o.warnings.Warning("input file was already specified.")
	}
	o.filename = joined.String()
	o.selected = true
	return nil
}
