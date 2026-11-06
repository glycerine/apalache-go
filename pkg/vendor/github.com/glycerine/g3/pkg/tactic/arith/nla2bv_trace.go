package arith

// Source: src/tactic/arith/nla2bv_tactic.cpp:85-88,111.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/text"
)

// BindTrace borrows an isolated trace stream. Nil restores the manager stream.
func (t *NLA2BVTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return nil
}

func (t *NLA2BVTactic) traceTags() *diagnostic.TraceTags {
	if t.trace != nil {
		return t.trace
	}
	return t.manager.BorrowMemoryTraceTags()
}

func nla2BVTraceWrite(out io.Writer, text string) error {
	n, err := io.WriteString(out, text)
	if err != nil {
		return err
	}
	if n != len(text) {
		return io.ErrShortWrite
	}
	return nil
}

// Source insertion keeps evaluating operands after ostream badbit. Only stream
// errors are ignored; operand construction errors interrupt the tactic.
type nla2BVTraceOstream struct{ out io.Writer }

func (w nla2BVTraceOstream) Write(p []byte) (int, error) {
	_, _ = w.out.Write(p)
	return len(p), nil
}

func (c *nla2BVInvocation) emitTrace(tag, method string, line int, body func(io.Writer) error) error {
	tags := c.t.traceTags()
	if !tags.ShouldTrace(tag) {
		return nil
	}
	err := tags.EmitConditional(tag, method, "src/tactic/arith/nla2bv_tactic.cpp", line,
		func() bool { return true }, func(w io.Writer) error { return body(nla2BVTraceOstream{w}) })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (c *nla2BVInvocation) traceGoalCount(g *tactic.Goal, line int, display bool) error {
	tags := c.t.traceTags()
	if !tags.ShouldTrace("nla2bv") {
		return nil
	}
	return c.emitTrace("nla2bv", "operator()", line, func(out io.Writer) error {
		if display {
			if err := g.Display(out); err != nil {
				return err
			}
		}
		count, err := c.countMultiplications(g)
		if err != nil {
			return err
		}
		return nla2BVTraceWrite(out, "Muls: "+strconv.FormatUint(uint64(count), 10)+"\n")
	})
}

func (c *nla2BVInvocation) traceGoalDisplay(g *tactic.Goal, tag string, line int, heading string) error {
	tags := c.t.traceTags()
	if !tags.ShouldTrace(tag) {
		return nil
	}
	return c.emitTrace(tag, "operator()", line, func(out io.Writer) error {
		if heading != "" {
			if err := nla2BVTraceWrite(out, heading); err != nil {
				return err
			}
		}
		return g.Display(out)
	})
}

func (c *nla2BVInvocation) tracePower2Substitution(key, value ast.ASTHandle) error {
	tags := c.t.traceTags()
	if !tags.ShouldTrace("nla2bv") {
		return nil
	}
	return c.emitTrace("nla2bv", "collect_power2", 135, func(out io.Writer) error {
		if err := c.writeTraceExpr(out, key); err != nil {
			return err
		}
		if err := nla2BVTraceWrite(out, " "); err != nil {
			return err
		}
		if err := c.writeTraceExpr(out, value); err != nil {
			return err
		}
		return nla2BVTraceWrite(out, "\n")
	})
}

func (c *nla2BVInvocation) traceExpressionLine(method string, line int, prefix string, expression ast.ASTHandle) error {
	return c.traceExpressionLines(method, line, prefix, []ast.ASTHandle{expression})
}

func (c *nla2BVInvocation) traceExpressionLines(method string, line int, prefix string, expressions []ast.ASTHandle) error {
	tags := c.t.traceTags()
	if !tags.ShouldTrace("nla2bv") {
		return nil
	}
	return c.emitTrace("nla2bv", method, line, func(out io.Writer) error {
		for _, expression := range expressions {
			if err := nla2BVTraceWrite(out, prefix); err != nil {
				return err
			}
			if err := c.writeTraceExpr(out, expression); err != nil {
				return err
			}
			if err := nla2BVTraceWrite(out, "\n"); err != nil {
				return err
			}
		}
		return nil
	})
}

func (c *nla2BVInvocation) verboseSatPreserving() error {
	enabled, err := diagnostic.ProcessVerboseEnabled(10)
	if err != nil || !enabled {
		return err
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	value := "false"
	if c.satPreserving {
		value = "true"
	}
	if err = stream.AppendCString("(nla->bv :sat-preserving " + value + ")\n"); err != nil {
		return err
	}
	return stream.Flush()
}

func (c *nla2BVInvocation) writeTraceExpr(out io.Writer, expression ast.ASTHandle) (err error) {
	manager := c.t.manager
	format, err := manager.SMT2PPExprTree(expression, nil)
	if err != nil {
		return err
	}
	formats, err := manager.GetFormatManager()
	if err != nil {
		return err
	}
	ref, err := formats.NewRef(format)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, formats.DelRef(&ref)) }()
	stream, err := text.NewNumericTextStream(text.NumericTextStreamConfig{Budget: manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, stream.Close()) }()
	empty := c.t.context.EmptyValue()
	if err = manager.PrintFormat(stream, format, c.t.context, &empty, c.t.globals); err != nil {
		return err
	}
	formatted, err := stream.String()
	if err != nil {
		return err
	}
	return nla2BVTraceWrite(out, formatted)
}
