package qe

// Source: src/muz/spacer/spacer_util.cpp qe_project TRACE checkpoints.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (p *LegacyProjection) traceProjectionStage(line int, formulaLabel string, formula ast.ASTRef, variablesLabel string, variables []ast.ASTHandle, md *model.ModelCore) error {
	tags := p.config.Trace
	if tags == nil {
		tags = p.manager.BorrowSourceTraceTags()
	}
	err := tags.EmitConditional("spacer_mbp", "qe_project", "src/muz/spacer/spacer_util.cpp", line,
		func() bool { return true }, func(out io.Writer) error {
			writer := traceBodyWriter{out}
			if md != nil {
				_, _ = io.WriteString(writer, "extended model:\n")
				if err := md.WriteDebug(writer, p.config.Context, p.config.Globals); err != nil {
					return err
				}
			}
			if formulaLabel != "" {
				_, _ = io.WriteString(writer, formulaLabel+"\n")
				handle, err := p.manager.RefGet(formula)
				if err != nil {
					return err
				}
				if err := writeExpression(p.manager, p.config, writer, handle); err != nil {
					return err
				}
				_, _ = io.WriteString(writer, "\n")
			}
			if variablesLabel != "" {
				_, _ = io.WriteString(writer, variablesLabel+"\n")
				for _, variable := range variables {
					if err := writeExpression(p.manager, p.config, writer, variable); err != nil {
						return err
					}
					_, _ = io.WriteString(writer, "\n")
				}
			}
			return nil
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
