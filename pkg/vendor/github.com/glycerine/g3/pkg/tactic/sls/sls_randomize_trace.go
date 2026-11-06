package sls

// Source: src/tactic/sls/sls_evaluator.h:808-810.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *Evaluator) traceRandomization(declaration ast.ASTHandle) error {
	tags := e.manager.BorrowSourceTraceTags()
	if !tags.ShouldTrace("sls") {
		return nil
	}
	if declaration.IsNull() {
		return ErrTracker
	}
	name, err := e.manager.GetASTName(declaration)
	if err != nil {
		return err
	}
	text, err := e.manager.Symbols.Display(name)
	if err != nil {
		return err
	}
	bodyErr := error(nil)
	err = tags.EmitConditional("sls", "randomize_local", "src/tactic/sls/sls_evaluator.h", 808,
		func() bool { return true }, func(out io.Writer) error {
			writer := slsTraceWriter{out}
			if _, writeErr := fmt.Fprintf(writer, "Randomization candidate: %s\n", text); writeErr != nil {
				bodyErr = writeErr
				return writeErr
			}
			if _, writeErr := io.WriteString(writer, "Locally randomized model: \n"); writeErr != nil {
				bodyErr = writeErr
				return writeErr
			}
			bodyErr = e.tracker.DisplayModel(writer)
			return bodyErr
		})
	if bodyErr != nil {
		return bodyErr
	}
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
