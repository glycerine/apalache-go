package core

// Source: src/tactic/core/symmetry_reduce_tactic.cpp live TRACE sites.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (t *SymmetryReduceTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return nil
}
func (t *SymmetryReduceTactic) emitSymmetry(function string, line int, body func(io.Writer) error) error {
	tags := t.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional("symmetry_reduce", function, "src/tactic/core/symmetry_reduce_tactic.cpp", line, func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (t *SymmetryReduceTactic) writeSymmetryExpr(out io.Writer, term ast.ASTHandle) (err error) {
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
func (t *SymmetryReduceTactic) writeSymmetrySet(out io.Writer, terms []ast.ASTHandle) error {
	for _, term := range terms {
		if err := t.writeSymmetryExpr(out, term); err != nil {
			return err
		}
		if _, err := io.WriteString(out, " "); err != nil {
			return err
		}
	}
	_, err := io.WriteString(out, "\n")
	return err
}
func (t *SymmetryReduceTactic) traceCandidates(constants []ast.ASTHandle) error {
	return t.emitSymmetry("find_candidate_permutations", 204, func(out io.Writer) error { return t.writeSymmetrySet(out, constants) })
}
func (t *SymmetryReduceTactic) traceSymmetry(constants []ast.ASTHandle, ok bool) error {
	return t.emitSymmetry("invariant_by_permutations", 427, func(out io.Writer) error {
		label := "Not symmetric: "
		if ok {
			label = "Symmetric: "
		}
		if _, err := io.WriteString(out, label); err != nil {
			return err
		}
		return t.writeSymmetrySet(out, constants)
	})
}
func (t *SymmetryReduceTactic) traceWeight(term ast.ASTHandle, weight uint32, delta, line int) error {
	return t.emitSymmetry("select_most_promising_term", line, func(out io.Writer) error {
		if err := t.writeSymmetryExpr(out, term); err != nil {
			return err
		}
		_, err := fmt.Fprintf(out, " %d %d\n", weight, delta)
		return err
	})
}
func (t *SymmetryReduceTactic) traceSupport(term ast.ASTHandle, constants, used []ast.ASTHandle) error {
	return t.emitSymmetry("compute_used_in", 587, func(out io.Writer) error {
		if _, err := io.WriteString(out, "Term: "); err != nil {
			return err
		}
		if err := t.writeSymmetryExpr(out, term); err != nil {
			return err
		}
		if _, err := io.WriteString(out, "\nSupport set: "); err != nil {
			return err
		}
		if err := t.writeSymmetrySet(out, constants); err != nil {
			return err
		}
		if _, err := io.WriteString(out, "Constants: "); err != nil {
			return err
		}
		return t.writeSymmetrySet(out, used)
	})
}
func (t *SymmetryReduceTactic) traceMember(term ast.ASTHandle) error {
	return t.emitSymmetry("operator()", 155, func(out io.Writer) error {
		if _, err := io.WriteString(out, "member predicate: "); err != nil {
			return err
		}
		if err := t.writeSymmetryExpr(out, term); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}
