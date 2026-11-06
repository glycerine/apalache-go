package qe

// Source: src/qe/qe_bool_plugin.cpp:65-66,136. These diagnostics use the
// source trace stream and the shared SMT2 expression printer.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *BoolPlugin) traceBoolProjection(config Config, target, value ast.ASTHandle) error {
	tags := config.Trace
	if tags == nil {
		tags = p.manager.BorrowSourceTraceTags()
	}
	err := tags.EmitConditional("qe", "project", "src/qe/qe_bool_plugin.cpp", 65,
		func() bool { return !p.manager.IsTrue(value) && !p.manager.IsFalse(value) }, func(out io.Writer) error {
			writer := traceBodyWriter{out}
			_, _ = io.WriteString(writer, "Boolean is a don't care: ")
			if err := writeExpression(p.manager, config, writer, target); err != nil {
				return err
			}
			_, _ = io.WriteString(writer, "\n")
			return nil
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (p *BoolPlugin) traceBoolPolarity(target, formula ast.ASTHandle) error {
	tags := p.manager.BorrowSourceTraceTags()
	if !tags.ShouldTrace("quant_elim") {
		return nil
	}
	context, globals, err := p.manager.RewriterParameterServices()
	if err != nil {
		return err
	}
	config := Config{Context: context, Globals: globals}
	err = tags.EmitConditional("quant_elim", "solve_polarized", "src/qe/qe_bool_plugin.cpp", 136,
		func() bool { return true }, func(out io.Writer) error {
			writer := traceBodyWriter{out}
			if err := writeExpression(p.manager, config, writer, target); err != nil {
				return err
			}
			_, _ = io.WriteString(writer, " ")
			if err := writeExpression(p.manager, config, writer, formula); err != nil {
				return err
			}
			_, _ = io.WriteString(writer, "\n")
			return nil
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
