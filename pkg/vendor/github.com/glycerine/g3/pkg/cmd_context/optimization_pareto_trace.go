package cmd_context

// Source: src/opt/opt_pareto.cpp:71-72,84-85.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// Source ostream failure does not suppress evaluation of later operands.
type paretoDiagnosticWriter struct{ out io.Writer }

func (w paretoDiagnosticWriter) Write(p []byte) (int, error) {
	_, _ = w.out.Write(p)
	return len(p), nil
}

type paretoVerboseWriter struct{ stream *utext.NumericTextStream }

func (w paretoVerboseWriter) Write(p []byte) (int, error) {
	if err := w.stream.AppendString(string(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Source: opt_pareto.cpp:43-46. Only this lexical model reference is fixed;
// the raw model remains available for the subsequent dominance callbacks.
func (c *Context) paretoModelDiagnostic(raw *model.ModelCore) (resultErr error) {
	enabled, err := diagnostic.ProcessVerboseEnabled(1)
	if err != nil || !enabled {
		return err
	}
	fixed, err := c.filterParetoModel(raw)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, fixed.DecRef()) }()
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	out := paretoDiagnosticWriter{paretoVerboseWriter{stream}}
	_, _ = io.WriteString(out, "new model:\n")
	return fixed.WriteSMT2(out, c.config.Params, c.config.Globals, 0)
}

func (c *Context) writeParetoDiagnosticExpression(out io.Writer, expression ast.ASTHandle) (resultErr error) {
	document, err := c.manager.SMT2PPExprTree(expression, nil)
	if err != nil {
		return err
	}
	formats, err := c.manager.GetFormatManager()
	if err != nil {
		return err
	}
	ref, err := formats.NewRef(document)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, formats.DelRef(&ref)) }()
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: c.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, stream.Close()) }()
	empty := c.config.Params.EmptyValue()
	if err = c.manager.PrintFormat(stream, document, c.config.Params, &empty, c.config.Globals); err != nil {
		return err
	}
	text, err := stream.String()
	if err != nil {
		return err
	}
	_, _ = io.WriteString(out, text)
	return nil
}

func (c *Context) paretoFormulaDiagnostics(formula ast.ASTHandle, md *model.ModelCore, dominates bool) error {
	label, method, line := "not dominated by: ", "mk_not_dominated_by", 85
	if dominates {
		label, method, line = "dominates: ", "mk_dominates", 72
	}
	enabled, err := diagnostic.ProcessVerboseEnabled(10)
	if err != nil {
		return err
	}
	if enabled {
		stream, err := diagnostic.BorrowProcessVerboseStream()
		if err != nil {
			return err
		}
		out := paretoDiagnosticWriter{paretoVerboseWriter{stream}}
		_, _ = io.WriteString(out, label)
		if err = c.writeParetoDiagnosticExpression(out, formula); err != nil {
			return err
		}
		_, _ = io.WriteString(out, "\n")
		// The operands above propagate construction failures. Flush only writes
		// the source ostream and does not turn its badbit into a solver failure.
		_ = stream.Flush()
	}
	tags := c.config.ModelEvaluator.Trace
	if tags == nil {
		tags = c.manager.BorrowMemoryTraceTags()
	}
	if !tags.ShouldTrace("opt") {
		return nil
	}
	err = tags.EmitConditional("opt", method, "src/opt/opt_pareto.cpp", line, func() bool { return true }, func(w io.Writer) error {
		out := paretoDiagnosticWriter{w}
		if err := c.writeParetoDiagnosticExpression(out, formula); err != nil {
			return err
		}
		_, _ = io.WriteString(out, "\n")
		if dominates {
			return md.WriteSMT2(out, c.config.Params, c.config.Globals, 0)
		}
		return nil
	})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
