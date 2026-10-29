package arith

// Source: src/tactic/arith/diff_neq_tactic.cpp live display/TRACE sites.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (t *DiffNeqTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return nil
}
func (t *DiffNeqTactic) emitDiffNeq(tag, function string, line int, body func(io.Writer) error) error {
	tags := t.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional(tag, function, "src/tactic/arith/diff_neq_tactic.cpp", line, func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (t *DiffNeqTactic) writeDiffNeqExpr(out io.Writer, term ast.ASTHandle) (err error) {
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: t.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, stream.Close()) }()
	format, err := t.manager.SMT2PPExprTree(term, nil)
	if err != nil {
		return err
	}
	empty := t.context.EmptyValue()
	if err = t.manager.PrintFormat(stream, format, t.context, &empty, t.globals); err != nil {
		return err
	}
	body, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, body)
	return err
}
func (t *DiffNeqTactic) Display(out io.Writer) error {
	if err := t.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	for _, v := range t.current.variables {
		if _, err := fmt.Fprintf(out, "%d <= ", v.lower); err != nil {
			return err
		}
		if err := t.writeDiffNeqExpr(out, v.term); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, " <= %d\n", v.upper); err != nil {
			return err
		}
	}
	for _, v := range t.current.variables {
		for _, d := range v.disequalities {
			if int(d.variable) >= len(t.current.variables) {
				return ast.ErrASTDomain
			}
			if err := t.writeDiffNeqExpr(out, v.term); err != nil {
				return err
			}
			if _, err := io.WriteString(out, " != "); err != nil {
				return err
			}
			if err := t.writeDiffNeqExpr(out, t.current.variables[d.variable].term); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(out, " + %d\n", d.offset); err != nil {
				return err
			}
		}
	}
	return nil
}
func (t *DiffNeqTactic) displayDiffNeqModel(out io.Writer) error {
	for i, value := range t.current.stack {
		if i >= len(t.current.variables) {
			return ast.ErrASTDomain
		}
		if err := t.writeDiffNeqExpr(out, t.current.variables[i].term); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, " := %d\n", value); err != nil {
			return err
		}
	}
	return nil
}
func (t *DiffNeqTactic) traceDiffNeqProcessing(term ast.ASTHandle) error {
	return t.emitDiffNeq("diff_neq_tactic", "compile", 171, func(out io.Writer) error {
		if _, err := io.WriteString(out, "processing: "); err != nil {
			return err
		}
		if err := t.writeDiffNeqExpr(out, term); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}
func (t *DiffNeqTactic) traceCompiledDiffNeq(goal *tactic.Goal) error {
	return t.emitDiffNeq("diff_neq_tactic", "operator()", 331, func(out io.Writer) error {
		if err := goal.Display(out); err != nil {
			return err
		}
		return t.Display(out)
	})
}
func (t *DiffNeqTactic) traceDiffNeqAssignment(line int) error {
	return t.emitDiffNeq("diff_neq_tactic", "search", line, t.displayDiffNeqModel)
}
func (t *DiffNeqTactic) traceDiffNeqResult(goal *tactic.Goal) error {
	return t.emitDiffNeq("diff_neq", "operator()", 344, goal.Display)
}
