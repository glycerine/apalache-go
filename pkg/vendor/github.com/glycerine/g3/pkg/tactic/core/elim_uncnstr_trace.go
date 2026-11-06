package core

// Source: src/tactic/core/elim_uncnstr_tactic.cpp live diagnostic sites.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (t *EliminateUnconstrainedTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return nil
}
func (t *EliminateUnconstrainedTactic) emit(tag, function string, line int, body func(io.Writer) error) error {
	tags := t.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional(tag, function, "src/tactic/core/elim_uncnstr_tactic.cpp", line, func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (t *EliminateUnconstrainedTactic) writeExpression(out io.Writer, term ast.ASTHandle, bounded bool) (err error) {
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: t.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, stream.Close()) }()
	if bounded {
		err = t.manager.DisplayBoundedLowLevel(stream, term, 3)
	} else {
		format, e := t.manager.SMT2PPExprTree(term, nil)
		if e != nil {
			return e
		}
		empty := t.context.EmptyValue()
		err = t.manager.PrintFormat(stream, format, t.context, &empty, t.globals)
	}
	if err != nil {
		return err
	}
	body, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, body)
	return err
}
func (t *EliminateUnconstrainedTactic) traceFresh(term ast.ASTHandle) error {
	for _, site := range []struct {
		tag     string
		line    int
		bounded bool
	}{{"elim_uncnstr_bug", 99, false}, {"elim_uncnstr_bug_ll", 100, true}} {
		if err := t.emit(site.tag, "mk_fresh_uncnstr_var_for", site.line, func(out io.Writer) error {
			if _, err := io.WriteString(out, "eliminating:\n"); err != nil {
				return err
			}
			if err := t.writeExpression(out, term, site.bounded); err != nil {
				return err
			}
			_, err := io.WriteString(out, "\n")
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}
func (t *EliminateUnconstrainedTactic) traceVariables() error {
	return t.emit("elim_uncnstr", "operator()", 838, func(out io.Writer) error {
		if _, err := io.WriteString(out, "unconstrained variables...\n"); err != nil {
			return err
		}
		for _, v := range t.orderedVariables {
			if err := t.writeExpression(out, v, false); err != nil {
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
func (t *EliminateUnconstrainedTactic) traceGoal(goal *tactic.Goal) error {
	return t.emit("elim_uncnstr_bug", "operator()", 828, goal.Display)
}
func (t *EliminateUnconstrainedTactic) traceConverter(converter tactic.ModelConverter) error {
	return t.emit("elim_uncnstr", "operator()", 891, func(out io.Writer) error {
		if converter == nil {
			return nil
		}
		return converter.Display(out)
	})
}
