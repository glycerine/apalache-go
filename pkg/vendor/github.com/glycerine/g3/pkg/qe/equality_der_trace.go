package qe

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (d *EqualityDER) traceConjunctions(function string, line int, prefix string, conjs []ast.ASTRef) error {
	return d.emit("qe_lite", function, line, func(out io.Writer) (err error) {
		args, err := d.expressions(conjs)
		if err != nil {
			return err
		}
		h, err := d.manager.MkAndArray(args)
		if err != nil {
			return err
		}
		ref, err := d.manager.NewRef(h)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, d.manager.DelRef(&ref)) }()
		_, _ = io.WriteString(out, prefix)
		if err = writeExpression(d.manager, d.config, out, h); err != nil {
			return err
		}
		_, _ = io.WriteString(out, "\n")
		return nil
	})
}
func (d *EqualityDER) traceUnconstrained(h, left, right ast.ASTHandle) error {
	return d.emit("qe_lite", "remove_unconstrained", 719, func(out io.Writer) error {
		if err := writeExpression(d.manager, d.config, out, h); err != nil {
			return err
		}
		for _, arg := range []ast.ASTHandle{left, right} {
			selected, err := d.selection.IsVariable(arg)
			if err != nil {
				return err
			}
			text := " 0"
			if selected {
				text = " 1"
			}
			_, _ = io.WriteString(out, text)
		}
		_, _ = io.WriteString(out, "\n")
		return nil
	})
}
