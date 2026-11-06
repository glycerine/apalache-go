package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:668-679,762; config_hwf.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (c *HardwareContext) polynomialInteger(p *HardwarePolynomial) (bool, error) {
	for i := range p.terms {
		term := &p.terms[i]
		if !c.isInt[term.variable] || !c.numbers.IsInteger(&term.coefficient) {
			if err := c.traceNonintegerPolynomialTerm(p, i); err != nil {
				return false, err
			}
			return false, nil
		}
	}
	return c.numbers.IsInteger(&p.constant), nil
}

func (c *HardwareContext) polynomialTraceTags() *diagnostic.TraceTags {
	if c.trace != nil {
		return c.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

// BindTrace borrows one tag stream for the concrete hardware context. Nil
// restores the source process-global stream.
func (c *HardwareContext) BindTrace(tags *diagnostic.TraceTags) error {
	if err := c.open(); err != nil {
		return err
	}
	c.trace = tags
	return nil
}

func (c *HardwareContext) traceNonintegerPolynomialTerm(p *HardwarePolynomial, i int) error {
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
			if err := hardwareWriteVariable(variableWriter, p.terms[i].variable, c.displayVariable); err != nil && (variableWriter.err == nil || !errors.Is(err, variableWriter.err)) {
				bodyErr = err
				return err
			}
			_, _ = io.WriteString(w, " ")
			value, err := c.numbers.ToString(&p.terms[i].coefficient)
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

func (c *HardwareContext) tracePolynomialInteger(p *HardwarePolynomial) error {
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
