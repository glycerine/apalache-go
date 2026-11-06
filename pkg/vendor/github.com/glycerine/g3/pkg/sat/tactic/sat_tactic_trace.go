package tactic

// Source: src/sat/tactic/sat_tactic.cpp:51,58,68,95,99,111,198.

import (
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func emitSATTrace(tags *diagnostic.TraceTags, tag, function string, line int, body func(io.Writer) error) error {
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional(tag, function, "src/sat/tactic/sat_tactic.cpp", line, func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (t *SATTactic) emit(tag, function string, line int, body func(io.Writer) error) error {
	return emitSATTrace(t.services.Trace, tag, function, line, body)
}
func (t *SATTactic) traceInterpreted(tr *GoalTranslator, interpreted bool) error {
	return t.emit("sat_solver_unknown", "operator()", 58, func(w io.Writer) error {
		flag := 0
		if interpreted {
			flag = 1
		}
		if _, err := fmt.Fprintf(w, "interpreted_atoms: %d\n", flag); err != nil {
			return err
		}
		entries, err := tr.atomEntries()
		if err != nil {
			return err
		}
		for _, entry := range entries {
			atom := entry.Expr
			constant, err := tr.isUninterpretedConstant(atom)
			if err != nil {
				return err
			}
			if constant {
				continue
			}
			text, err := tr.traceGoalFormula(atom)
			if err != nil {
				return err
			}
			if _, err = io.WriteString(w, text+"\n"); err != nil {
				return err
			}
		}
		return nil
	})
}
func (t *SATTactic) traceDIMACS(s *sat.Solver) error {
	return t.emit("sat_dimacs", "operator()", 68, func(w io.Writer) (err error) {
		stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: t.context.Budget()})
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, stream.Close()) }()
		if err = s.DisplayDIMACS(stream); err != nil {
			return err
		}
		text, err := stream.String()
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, text)
		return err
	})
}
func (t *SATTactic) writeDIMACSAtoms(tr *GoalTranslator) error {
	entries, err := tr.atomEntries()
	if err != nil {
		return err
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].Variable < entries[right].Variable })
	for _, entry := range entries {
		atom, v := entry.Expr, entry.Variable
		text, err := tr.traceGoalFormula(atom)
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintf(t.output, "c %d %s\n", v, text); err != nil {
			return err
		}
	}
	return nil
}
func (tr *GoalTranslator) traceSATModelValues(values sat.Model) error {
	return emitSATTrace(tr.traceTags, "sat_tactic", "operator()", 95, func(w io.Writer) error {
		for i, v := range values {
			if _, err := fmt.Fprintf(w, "%d:%s ", i, v.String()); err != nil {
				return err
			}
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}
func (tr *GoalTranslator) traceSATModelAtom(atom ast.ASTHandle, v uint32) error {
	return emitSATTrace(tr.traceTags, "sat_tactic", "operator()", 99, func(w io.Writer) error {
		text, err := tr.traceGoalFormula(atom)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "extracting value of %s\nvar: %d\n", text, v)
		return err
	})
}
func (tr *GoalTranslator) traceSATModel(md *model.ModelCore) error {
	return emitSATTrace(tr.traceTags, "sat_tactic", "operator()", 111, func(w io.Writer) error {
		g := uparams.BorrowProcessGlobalParamsService()
		if tr.printer != nil {
			g = tr.printer
		}
		if g == nil || g.Context == nil {
			return uparams.ErrGlobalParamsOwner
		}
		return md.WriteDebug(w, g.Context, g)
	})
}
