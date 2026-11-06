package core

// Source: src/tactic/core/solve_eqs_tactic.cpp diagnostic sites.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (t *SolveEquationsTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return nil
}

func (t *SolveEquationsTactic) emit(tag, function string, line int, body func(io.Writer) error) error {
	tags := t.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional(tag, function, "src/tactic/core/solve_eqs_tactic.cpp", line, func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (t *SolveEquationsTactic) writeExpression(out io.Writer, term ast.ASTHandle) (err error) {
	format, err := t.manager.SMT2PPExprTree(term, nil)
	if err != nil {
		return err
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: t.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, stream.Close()) }()
	empty := t.context.EmptyValue()
	if err = t.manager.PrintFormat(stream, format, t.context, &empty, t.globals); err != nil {
		return err
	}
	text, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, text)
	return err
}

func (t *SolveEquationsTactic) traceExpression(tag, function string, line int, prefix string, term ast.ASTHandle) error {
	return t.emit(tag, function, line, func(out io.Writer) error {
		if _, err := io.WriteString(out, prefix); err != nil {
			return err
		}
		if err := t.writeExpression(out, term); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}

func (t *SolveEquationsTactic) traceOccurrences(term ast.ASTHandle, num uint32) error {
	return t.emit("solve_eqs_check_occs", "check_occs", 90, func(out io.Writer) error {
		if err := t.writeExpression(out, term); err != nil {
			return err
		}
		_, err := fmt.Fprintf(out, " num_occs: %d max: %d\n", num, t.maxOccurrences)
		return err
	})
}

func (t *SolveEquationsTactic) traceVariables(function string, line int, prefix string, variables []ast.ASTHandle) error {
	return t.emit("solve_eqs", function, line, func(out io.Writer) error {
		if _, err := io.WriteString(out, prefix); err != nil {
			return err
		}
		for _, variable := range variables {
			if err := t.writeExpression(out, variable); err != nil {
				return err
			}
			if _, err := io.WriteString(out, " "); err != nil {
				return err
			}
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}

func (t *SolveEquationsTactic) traceNormalized() error {
	return t.emit("solve_eqs", "normalize", 606, func(out io.Writer) error {
		if _, err := io.WriteString(out, "after normalizing variables\n"); err != nil {
			return err
		}
		for _, variable := range t.ordered {
			definition, _, _, err := t.find(t.normalized, variable)
			if err != nil {
				return err
			}
			if err = t.writeExpression(out, variable); err != nil {
				return err
			}
			if _, err = io.WriteString(out, "\n----->\n"); err != nil {
				return err
			}
			if err = t.writeExpression(out, definition); err != nil {
				return err
			}
			if _, err = io.WriteString(out, "\n\n"); err != nil {
				return err
			}
		}
		return nil
	})
}

func (t *SolveEquationsTactic) traceSubstitution(before, after ast.ASTHandle) error {
	return t.emit("solve_eqs_subst", "substitute", 654, func(out io.Writer) error {
		if err := t.writeExpression(out, before); err != nil {
			return err
		}
		if _, err := io.WriteString(out, "\n--->\n"); err != nil {
			return err
		}
		if err := t.writeExpression(out, after); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}

func (t *SolveEquationsTactic) traceGoal(tag, function string, line int, prefix string, goal *tactic.Goal) error {
	return t.emit(tag, function, line, func(out io.Writer) error {
		if _, err := io.WriteString(out, prefix); err != nil {
			return err
		}
		return goal.Display(out)
	})
}

func (t *SolveEquationsTactic) traceRound(goal *tactic.Goal, converter *tactic.ExtensionModelConverter) error {
	return t.emit("solve_eqs_round", "operator()", 775, func(out io.Writer) error {
		if err := goal.Display(out); err != nil {
			return err
		}
		if converter != nil {
			return converter.Display(out)
		}
		return nil
	})
}

func (t *SolveEquationsTactic) reportEliminations() error {
	enabled, err := diagnostic.ProcessVerboseEnabled(100)
	if err != nil || !enabled || len(t.ordered) == 0 {
		return err
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = stream.AppendCString("num. eliminated vars: "); err != nil {
		return err
	}
	if err = stream.AppendUint32(uint32(len(t.ordered))); err != nil {
		return err
	}
	return stream.AppendCString("\n")
}
