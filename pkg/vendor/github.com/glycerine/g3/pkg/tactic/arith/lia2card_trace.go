package arith

// Source: src/tactic/arith/lia2card_tactic.cpp TRACE sites.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func lia2cardTraceTags(tags *diagnostic.TraceTags) *diagnostic.TraceTags {
	if tags == nil {
		return diagnostic.BorrowProcessTraceTags()
	}
	return tags
}

func lia2cardEmit(tags *diagnostic.TraceTags, tag, function string, line int, body func(io.Writer) error) error {
	tags = lia2cardTraceTags(tags)
	if !tags.ShouldTrace(tag) {
		return nil
	}
	err := tags.EmitConditional(tag, function, "src/tactic/arith/lia2card_tactic.cpp", line, func() bool { return true }, func(out io.Writer) error { return body(nla2BVTraceOstream{out}) })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (t *LIA2CardTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return t.bounds.BindTrace(tags)
}

func (t *LIA2CardTactic) traceBound(variable ast.ASTHandle) error {
	return lia2cardEmit(t.trace, "pb", "operator()", 185, func(out io.Writer) error {
		if _, err := io.WriteString(out, "add bound "); err != nil {
			return err
		}
		if err := ast.DisplayLegacySMT(out, t.manager, variable, 0, nil); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}

func lia2cardTraceUnsupported(m *ast.ASTManager, tags *diagnostic.TraceTags, expression ast.ASTHandle) error {
	return lia2cardEmit(tags, "pb", "get_sum", 342, func(out io.Writer) error {
		if _, err := io.WriteString(out, "Can't handle "); err != nil {
			return err
		}
		if err := ast.DisplayLegacySMT(out, m, expression, 0, nil); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}

func (t *LIA2CardTactic) traceComparison(decl ast.ASTHandle, args ast.AppArguments, result ast.ASTHandle) error {
	if !lia2cardTraceTags(t.trace).ShouldTrace("pbsum") {
		return nil
	}
	return lia2cardEmit(t.trace, "pbsum", "mk_app_core", 92, func(out io.Writer) (err error) {
		arguments := make([]ast.ASTHandle, args.Count())
		for i := range arguments {
			arguments[i], err = args.At(uint32(i))
			if err != nil {
				return err
			}
		}
		original, err := t.manager.MkApp(decl, arguments)
		if err != nil {
			return err
		}
		ref, err := t.manager.NewRef(original)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, t.manager.DelRef(&ref)) }()
		if err = t.bounds.writeExpr(out, original); err != nil {
			return err
		}
		if _, err = io.WriteString(out, " ==>\n"); err != nil {
			return err
		}
		if err = t.bounds.writeExpr(out, result); err != nil {
			return err
		}
		_, err = io.WriteString(out, "\n")
		return err
	})
}

func (t *LIA2CardTactic) traceGoal(goal *tactic.Goal) error {
	return lia2cardEmit(t.trace, "pb", "operator()", 210, goal.Display)
}
