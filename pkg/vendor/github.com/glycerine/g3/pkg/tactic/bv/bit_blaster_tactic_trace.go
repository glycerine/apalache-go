package bv

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (t *BitBlasterTactic) emit(tag string, line int, body func(io.Writer) error) error {
	tags := t.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional(tag, "operator()", "src/tactic/bv/bit_blaster_tactic.cpp", line, func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (t *BitBlasterTactic) writeExpr(w io.Writer, h ast.ASTHandle) (err error) {
	f, err := t.manager.SMT2PPExprTree(h, nil)
	if err != nil {
		return err
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: t.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, stream.Close()) }()
	p := t.context.EmptyValue()
	if err = t.manager.PrintFormat(stream, f, t.context, &p, t.globals); err != nil {
		return err
	}
	s, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, s)
	return err
}
