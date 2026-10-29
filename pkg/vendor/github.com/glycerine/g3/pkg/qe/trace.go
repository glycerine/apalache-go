package qe

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// Config borrows the concrete parameter/diagnostic and cooperation services
// used by QE algorithms. Those services outlive the algorithm owners.
type Config struct {
	Context     *uparams.ParamsContext
	Globals     *uparams.GlobalParamsService
	Cooperation *cooperate.Cooperation
	Trace       *diagnostic.TraceTags
}

// The enclosing TRACE owns ostream's sticky state. A failed insertion must
// still evaluate subsequent source pretty-printer operands.
type traceBodyWriter struct{ out io.Writer }

func (w traceBodyWriter) Write(p []byte) (int, error) { _, _ = w.out.Write(p); return len(p), nil }

func (w traceBodyWriter) Flush() error {
	if out, ok := w.out.(interface{ Flush() error }); ok {
		_ = out.Flush()
	}
	return nil
}

func diagnosticEndl(out io.Writer) error { return diagnostic.WriteTraceEndl(out) }

// ast_smt2_pp(expr_ref_vector) uses mk_seq: a forced line break precedes
// every expression, with no enclosing delimiters and no empty-vector text.
func writeExpressionVector(m *ast.ASTManager, config Config, out io.Writer, forms []ast.ASTRef) error {
	for _, r := range forms {
		if _, err := io.WriteString(out, "\n"); err != nil {
			return err
		}
		h, err := m.RefGet(r)
		if err != nil {
			return err
		}
		if err = writeExpression(m, config, out, h); err != nil {
			return err
		}
	}
	return nil
}

func writeExpression(m *ast.ASTManager, config Config, out io.Writer, expression ast.ASTHandle) (err error) {
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, stream.Close()) }()
	f, err := m.SMT2PPExprTree(expression, nil)
	if err != nil {
		return err
	}
	fm, err := m.GetFormatManager()
	if err != nil {
		return err
	}
	ref, err := fm.NewRef(f)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, fm.DelRef(&ref)) }()
	empty := config.Context.EmptyValue()
	if err = m.PrintFormat(stream, f, config.Context, &empty, config.Globals); err != nil {
		return err
	}
	text, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, text)
	return err
}

func (d *ArrayDER) traceSelect(left, right ast.ASTHandle) error {
	tags := d.config.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional("qe_lite", "solve_select", "src/qe/qe_lite.cpp", 889, func() bool { return true }, func(out io.Writer) error {
		writer := traceBodyWriter{out}
		if err := writeExpression(d.manager, d.config, writer, left); err != nil {
			return err
		}
		_, _ = io.WriteString(writer, " = ")
		if err := writeExpression(d.manager, d.config, writer, right); err != nil {
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
