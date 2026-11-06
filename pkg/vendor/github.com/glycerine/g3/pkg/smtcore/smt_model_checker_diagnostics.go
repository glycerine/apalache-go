package smtcore

// Source: src/smt/smt_model_checker.cpp check/check_quantifiers/restart_eh.

import (
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// MBQI trace is selected by its own parameter, independently of verbosity.
// Both source paths write to the borrowed process verbose stream.
func writeMBQIDiagnostic(message string) error {
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = out.AppendCString(message); err != nil {
		return err
	}
	return out.Flush()
}

func (c *SMTModelChecker) reportQuantifier(event string, q ast.ASTHandle, namedOnly bool) error {
	if !c.trace {
		if namedOnly {
			return nil
		}
		enabled, err := diagnostic.ProcessVerboseEnabled(5)
		if err != nil || !enabled {
			return err
		}
	}
	id, _, err := c.manager.GetQuantifierIDs(q)
	if err != nil {
		return err
	}
	if namedOnly && id.IsNull() {
		return nil
	}
	name, err := c.manager.Symbols.Display(id)
	if err != nil {
		return err
	}
	return writeMBQIDiagnostic("(smt.mbqi :" + event + " " + name + ")\n")
}

func (c *SMTModelChecker) reportResult(failures uint32) error {
	if failures == 0 {
		return writeMBQIDiagnostic("(smt.mbqi :succeeded true)\n")
	}
	return writeMBQIDiagnostic("(smt.mbqi :num-failures " + strconv.FormatUint(uint64(failures), 10) + ")\n")
}

func (c *SMTModelChecker) reportIterationLimit() error {
	enabled, err := diagnostic.ProcessVerboseEnabled(1)
	if err != nil || !enabled {
		return err
	}
	return writeMBQIDiagnostic("(smt.mbqi \"max instantiations " + strconv.FormatUint(uint64(c.iteration), 10) + " reached\")\n")
}

func (c *SMTModelChecker) reportRestart() error {
	enabled, err := diagnostic.ProcessVerboseEnabled(100)
	if err != nil || !enabled {
		return err
	}
	return writeMBQIDiagnostic("(smt.mbqi \"instantiating new instances...\")\n")
}
