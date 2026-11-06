package bv

// Source: src/tactic/bv/bv_bound_chk_tactic.cpp live TRACE/CTRACE sites.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (t *BVBoundCheckTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return nil
}
func (t *BVBoundCheckTactic) emit(tag, function string, line int, body func(io.Writer) error) error {
	tags := t.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional(tag, function, "src/tactic/bv/bv_bound_chk_tactic.cpp", line, func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (t *BVBoundCheckTactic) emitGoal(g *tactic.Goal, before bool) error {
	line := 213
	label := "after:"
	if before {
		line = 208
		label = "before:"
	}
	return t.emit("bv-bound-chk", "operator()", line, func(w io.Writer) error {
		if _, err := io.WriteString(w, label); err != nil {
			return err
		}
		if err := g.Display(w); err != nil {
			return err
		}
		if before {
			_, err := io.WriteString(w, "\n")
			return err
		}
		return nil
	})
}
func (t *BVBoundCheckTactic) write(w io.Writer, h ast.ASTHandle) (err error) {
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: t.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, stream.Close()) }()
	format, err := t.manager.SMT2PPExprTree(h, nil)
	if err != nil {
		return err
	}
	fm, err := t.manager.GetFormatManager()
	if err != nil {
		return err
	}
	ref, err := fm.NewRef(format)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, fm.DelRef(&ref)) }()
	p := t.context.EmptyValue()
	if err = t.manager.PrintFormat(stream, format, t.context, &p, t.globals); err != nil {
		return err
	}
	s, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, s)
	return err
}
func (t *BVBoundCheckTactic) emitStep(d ast.ASTHandle, args ast.AppArguments, result ast.ASTHandle) error {
	return t.emit("bv_bound_chk_step", "reduce_app", 63, func(w io.Writer) error {
		name, err := t.manager.GetASTName(d)
		if err != nil {
			return err
		}
		text, err := t.manager.Symbols.String(name)
		if err != nil {
			return err
		}
		if _, err = io.WriteString(w, text+"\n"); err != nil {
			return err
		}
		for i := uint32(0); i < args.Count(); i++ {
			h, err := args.At(i)
			if err != nil {
				return err
			}
			if err = t.write(w, h); err != nil {
				return err
			}
			if _, err = io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		if _, err = io.WriteString(w, "---------->\n"); err != nil {
			return err
		}
		if err = t.write(w, result); err != nil {
			return err
		}
		_, err = io.WriteString(w, "\n")
		return err
	})
}
