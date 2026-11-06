package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:668-679,762.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type polynomialTraceVariableWriter struct {
	out io.Writer
	err error
}

func (w *polynomialTraceVariableWriter) Write(p []byte) (int, error) {
	n, err := w.out.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	if err != nil && w.err == nil {
		w.err = err
	}
	return n, err
}

// polynomialInteger is the concrete config_mpq is_int(polynomial) body.
// TRACE calls it once for display, then mk_var calls it again.
func (c *RationalContext) polynomialInteger(p *RationalPolynomial) (bool, error) {
	for i := range p.terms {
		term := &p.terms[i]
		if !c.variables[term.variable] {
			if err := c.traceNonintegerPolynomialTerm(p, i); err != nil {
				return false, err
			}
			return false, nil
		}
		ok, err := c.numbers.IsInt(&term.coefficient)
		if err != nil {
			return false, err
		}
		if !ok {
			if err := c.traceNonintegerPolynomialTerm(p, i); err != nil {
				return false, err
			}
			return false, nil
		}
	}
	return c.numbers.IsInt(&p.constant)
}

func (c *RationalContext) polynomialTraceTags() *diagnostic.TraceTags {
	if c.trace != nil {
		return c.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (c *RationalContext) traceNonintegerPolynomialTerm(p *RationalPolynomial, i int) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_is_int") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_is_int", "is_int", "src/math/subpaving/subpaving_t_def.h", 672,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "polynomial is not integer due to monomial at i: ")
			_, _ = io.WriteString(w, strconv.FormatUint(uint64(i), 10))
			_, _ = io.WriteString(w, "\n")
			if flush, ok := w.(interface{ Flush() error }); ok {
				_ = flush.Flush()
			}
			variableWriter := &polynomialTraceVariableWriter{out: w}
			if err := c.writeVariable(variableWriter, p.terms[i].variable); err != nil && (variableWriter.err == nil || !errors.Is(err, variableWriter.err)) {
				bodyErr = err
				return err
			}
			_, _ = io.WriteString(w, " ")
			value, err := c.numbers.String(&p.terms[i].coefficient)
			if err != nil {
				bodyErr = err
				return err
			}
			_, _ = io.WriteString(w, value)
			_, _ = io.WriteString(w, "\n")
			return nil
		})
	return bodyErr
}

// BindTrace borrows this concrete context's diagnostic stream. Nil restores
// the source process-global stream.
func (c *RationalContext) BindTrace(tags *diagnostic.TraceTags) error {
	if err := c.open(); err != nil {
		return err
	}
	c.trace = tags
	return nil
}

func (c *RationalContext) tracePolynomialInteger(p *RationalPolynomial) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_mk_sum") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_mk_sum", "mk_sum", "src/math/subpaving/subpaving_t_def.h", 762,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "new variable is integer: ")
			integer, err := c.polynomialInteger(p)
			if err != nil {
				bodyErr = err
				return err
			}
			if integer {
				_, _ = io.WriteString(w, "1")
			} else {
				_, _ = io.WriteString(w, "0")
			}
			_, _ = io.WriteString(w, "\n")
			return nil
		})
	return bodyErr
}
