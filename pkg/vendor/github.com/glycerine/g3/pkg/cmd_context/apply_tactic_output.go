package cmd_context

// Source: src/cmd_context/tactic_cmds.cpp:309-347, apply result views.

import (
	"io"

	"github.com/glycerine/g3/pkg/tactic"
)

func (c *Context) writeAppliedTacticResult(out io.Writer, applied *tactic.TacticResult, settings applyTacticSettings) error {
	if settings.print {
		if _, err := io.WriteString(out, "(goals\n"); err != nil {
			return err
		}
		for _, goal := range applied.Goals {
			var err error
			if settings.printDependencies {
				err = goal.DisplayWithDependencies(out)
			} else {
				err = goal.Display(out)
			}
			if err != nil {
				return err
			}
		}
		if _, err := io.WriteString(out, ")\n"); err != nil {
			return err
		}
	}
	// Source accepts print_proof but leaves its implementation as TODO.
	_ = settings.printProof
	if settings.printBenchmark {
		if err := c.writeAppliedTacticBenchmark(out, applied.Goals); err != nil {
			return err
		}
	}
	if settings.printModelConverter && applied.ModelConverter != nil {
		return applied.ModelConverter.Display(out)
	}
	return nil
}
