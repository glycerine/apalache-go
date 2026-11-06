package arith

// Source: src/tactic/arith/card2bv_tactic.cpp TRACE sites.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (t *Card2BVTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return nil
}
func (t *Card2BVTactic) emit(tag string, line int, body func(io.Writer) error) error {
	tags := t.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional(tag, "operator()", "src/tactic/arith/card2bv_tactic.cpp", line, func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (t *Card2BVTactic) traceGoal(tag string, line int, g *tactic.Goal) error {
	return t.emit(tag, line, g.Display)
}
func (t *Card2BVTactic) traceRewrite(h ast.ASTHandle) error {
	return t.emit("card2bv", 75, func(w io.Writer) (err error) {
		if _, err = io.WriteString(w, "Rewriting "); err != nil {
			return err
		}
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
		text, err := stream.String()
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, text+"\n")
		return err
	})
}
