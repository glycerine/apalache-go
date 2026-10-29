package smtcore

// Source: src/smt/theory_lra.cpp:2222-2223,2470-2502.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/lp"
)

func (t *SMTLRATheory) emitLRA(function string, line int, body func(io.Writer) error) error {
	return t.emitLRATag("arith", function, line, body)
}

func (t *SMTLRATheory) lraTraceTags() *diagnostic.TraceTags {
	tags := t.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	return tags
}

func (t *SMTLRATheory) emitLRATag(tag, function string, line int, body func(io.Writer) error) error {
	tags := t.lraTraceTags()
	var operandErr error
	err := tags.EmitConditional(tag, function, "src/smt/theory_lra.cpp", line, func() bool { return true }, func(out io.Writer) error {
		// Source ostream badbit does not prevent later operands from running.
		operandErr = body(assertedTraceWriter{out})
		return operandErr
	})
	if operandErr != nil {
		return operandErr
	}
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (t *SMTLRATheory) displayEvidenceLiteral(out io.Writer, literal SMTLiteral) (result error) {
	m := t.state.manager
	expression, err := t.state.literalExpression(literal)
	if err != nil {
		return err
	}
	ref, err := m.NewRef(expression)
	if err != nil {
		return err
	}
	defer func() {
		err := m.DelRef(&ref)
		if err != nil {
			t.pendingTraceRoots = append(t.pendingTraceRoots, ref)
		}
		result = errors.Join(result, err)
	}()
	printer, err := ast.NewSimpleASTPrinterContext(m, out, out)
	if err != nil {
		return err
	}
	return printer.DisplayExpr(out, expression, 0)
}

// Immediate registration has inequality sources only. The equality/definition
// producers are confined to the unreachable constructor-disabled delayed path.
func (t *SMTLRATheory) displayEvidence(out io.Writer, evidence []smtLRAEvidence) error {
	if _, err := t.State(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	for _, e := range evidence {
		if diagnostic.DebugBuild {
			d := diagnostic.BorrowProcessDebugContext()
			enabled, err := diagnostic.SourceAssertionsEnabled(d)
			if err != nil {
				return err
			}
			if enabled && e.coefficient.Sign() == 0 {
				if err = diagnostic.DebugAssertionFailure(d, "src/smt/theory_lra.cpp", 2473, "!ev.first.is_zero()"); err != nil {
					return err
				}
			}
		}
		if e.coefficient.Sign() == 0 {
			continue
		}
		if uint64(e.constraint) >= uint64(len(t.numeric.inequalities)) || t.numeric.inequalities[e.constraint] == SMTNullLiteral {
			return ast.ErrASTDomain
		}
		literal := t.numeric.inequalities[e.constraint]
		if err := t.displayEvidenceLiteral(out, literal); err != nil {
			return err
		}
		assignment, err := t.state.Assignment(literal)
		if err != nil {
			return err
		}
		if err = smtWriteTheoryText(out, " "+assignment.String()+"\n"); err != nil {
			return err
		}
	}
	for _, e := range evidence {
		if err := smtWriteTheoryText(out, fmt.Sprintf("%s: ", e.coefficient.RatString())); err != nil {
			return err
		}
		if err := t.numeric.solver.PrintConstraint(e.constraint, out); err != nil {
			return err
		}
	}
	return nil
}

// Source make_feasible clears values and publishes counters before the verbose
// display. Its term getter may populate zero cache entries before LP search.
func (t *SMTLRATheory) makeFeasible() (lbool.LBool, error) {
	t.modelValues = nil
	if err := t.numeric.startFeasibility(); err != nil {
		return lbool.Undef, err
	}
	if err := t.emitLRATag("arith_verbose", "make_feasible", 2150, t.Display); err != nil {
		return lbool.Undef, err
	}
	result, status, err := t.numeric.findFeasible()
	if err != nil {
		return lbool.Undef, err
	}
	if result == lbool.Undef {
		if err = t.emitLRA("make_feasible", 2166, func(out io.Writer) error {
			name, err := lp.StatusString(status)
			if err != nil {
				return err
			}
			return smtWriteTheoryText(out, "status treated as inconclusive: "+name+"\n")
		}); err != nil {
			return lbool.Undef, err
		}
	}
	return result, nil
}

func (t *SMTLRATheory) displayLRAExpression(out io.Writer, expression ast.ASTHandle) error {
	printer, err := ast.NewSimpleASTPrinterContext(t.state.manager, out, out)
	if err != nil {
		return err
	}
	return printer.DisplayExpr(out, expression, 0)
}

// Source TRACE operands are borrowed expressions. Disabled/native traces must
// not evaluate the printer or require an expression lookup.
func (t *SMTLRATheory) traceLRAExpression(function string, line int, prefix string, expression ast.ASTHandle, suffix string) error {
	return t.emitLRA(function, line, func(out io.Writer) error {
		if err := smtWriteTheoryText(out, prefix); err != nil {
			return err
		}
		if err := t.displayLRAExpression(out, expression); err != nil {
			return err
		}
		return smtWriteTheoryText(out, suffix)
	})
}

func (t *SMTLRATheory) updateUnassignedBounds(v SMTTheoryVar, increment int) error {
	if err := t.emitLRA("updt_unassigned_bounds", 595, func(out io.Writer) error {
		return smtWriteTheoryText(out, fmt.Sprintf("v%d %d += %d\n", v, t.unassigned[v], increment))
	}); err != nil {
		return err
	}
	// The native vector holds unsigned values. Preserve its arithmetic; the
	// enclosing context/scope already owns the restoration trail in Go.
	t.unassigned[v] += uint32(increment)
	return nil
}
