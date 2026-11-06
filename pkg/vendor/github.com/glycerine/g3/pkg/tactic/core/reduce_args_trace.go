package core

// Source: src/tactic/core/reduce_args_tactic.cpp TRACE sites.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (t *ReduceArgsTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return nil
}
func (t *ReduceArgsTactic) emit(function string, line int, body func(io.Writer) error) error {
	tags := t.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional("reduce_args", function, "src/tactic/core/reduce_args_tactic.cpp", line, func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (t *ReduceArgsTactic) emitGoal(g *tactic.Goal, mc tactic.ModelConverter, line int) error {
	return t.emit("operator()", line, func(w io.Writer) error {
		if err := g.Display(w); err != nil {
			return err
		}
		if mc != nil {
			return mc.Display(w)
		}
		return nil
	})
}
func (t *ReduceArgsTactic) emitCandidates(non map[ast.ASTHandle]bool, candidates map[ast.ASTHandle]*reduceArgsCandidate, line int) error {
	function := "populate_decl2args"
	if non != nil {
		function = "find_non_candidates"
	}
	return t.emit(function, line, func(w io.Writer) error {
		name := func(d ast.ASTHandle) (string, error) {
			symbol, err := t.manager.GetASTName(d)
			if err != nil {
				return "", err
			}
			return t.manager.Symbols.String(symbol)
		}
		if non != nil {
			if _, err := io.WriteString(w, "non_candidates:\n"); err != nil {
				return err
			}
			for d := range non {
				s, err := name(d)
				if err != nil {
					return err
				}
				if _, err = fmt.Fprintln(w, s); err != nil {
					return err
				}
			}
			return nil
		}
		if _, err := io.WriteString(w, "decl2args:\n"); err != nil {
			return err
		}
		for d, c := range candidates {
			s, err := name(d)
			if err != nil {
				return err
			}
			if _, err = fmt.Fprintf(w, "%s: ", s); err != nil {
				return err
			}
			for _, v := range c.mask {
				digit := "0"
				if v {
					digit = "1"
				}
				if _, err = io.WriteString(w, digit); err != nil {
					return err
				}
			}
			if _, err = io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		return nil
	})
}
