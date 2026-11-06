package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:668-679,762; config_mpfx.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// FixedVariableDisplay is borrowed from the caller. Nil restores the source
// default x0, x1, ... spelling.
type FixedVariableDisplay func(io.Writer, uint32) error

func (c *FixedContext) SetVariableDisplay(display FixedVariableDisplay) error {
	if err := c.open(); err != nil {
		return err
	}
	c.displayVariable = display
	return nil
}

func (c *FixedContext) writeVariable(w io.Writer, variable uint32) error {
	if c.displayVariable != nil {
		return c.displayVariable(w, variable)
	}
	value := "x" + strconv.FormatUint(uint64(variable), 10)
	n, err := io.WriteString(w, value)
	if err == nil && n != len(value) {
		return io.ErrShortWrite
	}
	return err
}

func (c *FixedContext) polynomialInteger(p *FixedPolynomial) (bool, error) {
	for i := range p.terms {
		term := &p.terms[i]
		if !c.isInt[term.Variable] {
			if err := c.traceNonintegerPolynomialTerm(p, i); err != nil {
				return false, err
			}
			return false, nil
		}
		ok, err := c.numbers.IsInt(term.Coefficient)
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
	return c.numbers.IsInt(p.constant)
}

func (c *FixedContext) polynomialTraceTags() *diagnostic.TraceTags {
	if c.trace != nil {
		return c.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

// BindTrace borrows one tag stream for the concrete fixed-point context. Nil
// restores the source process-global stream.
func (c *FixedContext) BindTrace(tags *diagnostic.TraceTags) error {
	if err := c.open(); err != nil {
		return err
	}
	c.trace = tags
	return nil
}

func (c *FixedContext) traceNonintegerPolynomialTerm(p *FixedPolynomial, i int) error {
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
			if err := c.writeVariable(variableWriter, p.terms[i].Variable); err != nil && (variableWriter.err == nil || !errors.Is(err, variableWriter.err)) {
				bodyErr = err
				return err
			}
			_, _ = io.WriteString(w, " ")
			value, err := c.numbers.String(p.terms[i].Coefficient)
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

func (c *FixedContext) tracePolynomialInteger(p *FixedPolynomial) error {
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
