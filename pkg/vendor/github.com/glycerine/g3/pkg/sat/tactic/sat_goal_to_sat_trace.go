package tactic

// Source: src/sat/tactic/goal2sat.cpp:88-103,132,192,204,244,282,325,380-422,480;
// src/util/trace.h:40-46.

import (
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (tr *GoalTranslator) goalTraceTags() *diagnostic.TraceTags {
	if tr.traceTags != nil {
		return tr.traceTags
	}
	return diagnostic.BorrowProcessTraceTags()
}

func goalLiteralText(values []sat.Literal, trailingSpace bool) string {
	var out strings.Builder
	for i, value := range values {
		if i != 0 {
			out.WriteByte(' ')
		}
		out.WriteString(value.String())
	}
	if trailingSpace && len(values) != 0 {
		out.WriteByte(' ')
	}
	return out.String()
}

func (tr *GoalTranslator) mkClause(values []sat.Literal, array bool) error {
	if err := tr.traceGoalClause(values, array); err != nil {
		return err
	}
	return tr.solver.MkClause(values)
}

func (tr *GoalTranslator) traceGoalClause(values []sat.Literal, array bool) error {
	tags := tr.goalTraceTags()
	if !tags.ShouldTrace("goal2sat") {
		return nil
	}
	line := 103
	if !array {
		switch len(values) {
		case 1:
			line = 88
		case 2:
			line = 93
		case 3:
			line = 98
		default:
			return ast.ErrASTDomain
		}
	}
	return tags.EmitConditional("goal2sat", "mk_clause", "src/sat/tactic/goal2sat.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			_, err := io.WriteString(w, "mk_clause: "+goalLiteralText(values, array)+"\n")
			return err
		})
}

func (tr *GoalTranslator) traceGoalStart(h ast.ASTHandle) error {
	tags := tr.goalTraceTags()
	if !tags.ShouldTrace("goal2sat") {
		return nil
	}
	return tags.EmitConditional("goal2sat", "process", "src/sat/tactic/goal2sat.cpp", 380,
		func() bool { return true }, func(w io.Writer) error {
			if _, err := io.WriteString(w, "converting: "); err != nil {
				return err
			}
			formula, err := tr.traceGoalFormula(h)
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, formula+"\n")
			return err
		})
}

func (tr *GoalTranslator) traceGoalRemaining() error {
	return tr.goalTraceTags().EmitConditional("goal2sat", "process", "src/sat/tactic/goal2sat.cpp", 421,
		func() bool { return len(tr.results) != 0 }, func(w io.Writer) error {
			_, err := io.WriteString(w, goalLiteralText(tr.results, false)+"\n")
			return err
		})
}

func (tr *GoalTranslator) traceGoalCompiledFormula(h ast.ASTHandle) error {
	tags := tr.goalTraceTags()
	if !tags.ShouldTrace("sat") {
		return nil
	}
	return tags.EmitConditional("sat", "operator()", "src/sat/tactic/goal2sat.cpp", 480,
		func() bool { return true }, func(w io.Writer) error {
			formula, err := tr.traceGoalFormula(h)
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, formula+"\n")
			return err
		})
}

func (tr *GoalTranslator) traceGoalNewVar(v sat.BoolVar, h ast.ASTHandle) error {
	tags := tr.goalTraceTags()
	if !tags.ShouldTrace("goal2sat") {
		return nil
	}
	return tags.EmitConditional("goal2sat", "convert_atom", "src/sat/tactic/goal2sat.cpp", 132,
		func() bool { return true }, func(w io.Writer) error {
			if _, err := io.WriteString(w, "new_var: "+strconv.FormatUint(uint64(v), 10)+"\n"); err != nil {
				return err
			}
			formula, err := tr.traceGoalFormula(h)
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, formula+"\n")
			return err
		})
}

// expr2var::insert traces interpreted atoms before retaining the AST key.
func (tr *GoalTranslator) traceInterpretedAtom(h ast.ASTHandle) error {
	tags := tr.goalTraceTags()
	if !tags.ShouldTrace("expr2var") {
		return nil
	}
	return tags.EmitConditional("expr2var", "insert", "src/ast/expr2var.cpp", 29,
		func() bool { return true }, func(w io.Writer) error {
			if _, err := io.WriteString(w, "interpreted:\n"); err != nil {
				return err
			}
			formula, err := tr.traceGoalFormula(h)
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, formula+"\n")
			return err
		})
}

func (tr *GoalTranslator) traceGoalUnhandled(h ast.ASTHandle) error {
	tags := tr.goalTraceTags()
	if !tags.ShouldTrace("goal2sat_not_handled") {
		return nil
	}
	return tags.EmitConditional("goal2sat_not_handled", "visit", "src/sat/tactic/goal2sat.cpp", 192,
		func() bool { return true }, func(w io.Writer) error {
			formula, err := tr.traceGoalFormula(h)
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, formula+"\n")
			return err
		})
}

func (tr *GoalTranslator) traceGoalConversion(op int32, fr satGoalFrame) error {
	line, function, prefix := 0, "", ""
	switch op {
	case ast.ASTOpOr:
		line, function, prefix = 204, "convert_or", "convert_or:\n"
	case ast.ASTOpAnd:
		line, function, prefix = 244, "convert_and", "convert_and:\n"
	case ast.ASTOpIff, ast.ASTOpEq:
		root, sign := "0", "0"
		if fr.root {
			root = "1"
		}
		if fr.sign {
			sign = "1"
		}
		line, function, prefix = 325, "convert_iff", "convert_iff "+root+" "+sign+"\n"
	default:
		return nil
	}
	tags := tr.goalTraceTags()
	if !tags.ShouldTrace("goal2sat") {
		return nil
	}
	return tags.EmitConditional("goal2sat", function, "src/sat/tactic/goal2sat.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			if _, err := io.WriteString(w, prefix); err != nil {
				return err
			}
			formula, err := tr.traceGoalFormula(fr.term)
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, formula+"\n")
			return err
		})
}

func (tr *GoalTranslator) traceGoalAndResult() error {
	tags := tr.goalTraceTags()
	if !tags.ShouldTrace("goal2sat") {
		return nil
	}
	return tags.EmitConditional("goal2sat", "convert_and", "src/sat/tactic/goal2sat.cpp", 282,
		func() bool { return true }, func(w io.Writer) error {
			_, err := io.WriteString(w, goalLiteralText(tr.results, false)+"\n")
			return err
		})
}

func (tr *GoalTranslator) traceGoalFormula(h ast.ASTHandle) (body string, result error) {
	global := uparams.BorrowProcessGlobalParamsService()
	if tr.printer != nil {
		global = tr.printer
	}
	if global == nil || global.Context == nil {
		return "", uparams.ErrGlobalParamsOwner
	}
	format, err := tr.manager.SMT2PPExprTree(h, nil)
	if err != nil {
		return "", err
	}
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: tr.manager.BudgetField})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	empty := global.Context.EmptyValue()
	if err = tr.manager.PrintFormat(out, format, global.Context, &empty, global); err != nil {
		return "", err
	}
	return out.String()
}

func (tr *GoalTranslator) traceGoalResultStack(line int, label string, fr satGoalFrame) error {
	tags := tr.goalTraceTags()
	if !tags.ShouldTrace("goal2sat_bug") {
		return nil
	}
	return tags.EmitConditional("goal2sat_bug", "process", "src/sat/tactic/goal2sat.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			if _, err := io.WriteString(w, label); err != nil {
				return err
			}
			formula, err := tr.traceGoalFormula(fr.term)
			if err != nil {
				return err
			}
			root, sign := "0", "0"
			if fr.root {
				root = "1"
			}
			if fr.sign {
				sign = "1"
			}
			if _, err = io.WriteString(w, formula+" root: "+root+" sign: "+sign+"\n"); err != nil {
				return err
			}
			_, err = io.WriteString(w, goalLiteralText(tr.results, false)+"\n")
			return err
		})
}
