package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1780-1801; config_mpfx.

import "io"

func (c *FixedContext) traceInitialUnit(a *FixedInequality, axiom bool) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_init") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_init", "assert_units", "src/math/subpaving/subpaving_t_def.h", 1780,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, "asserting: ")
			if err := c.displayInequality(out, a); err != nil {
				bodyErr = err
				return err
			}
			if axiom {
				_, _ = io.WriteString(out, ", axiom: 1\n")
			} else {
				_, _ = io.WriteString(out, ", axiom: 0\n")
			}
			return nil
		})
	return bodyErr
}

func (c *FixedContext) traceInitialBounds(n *FixedNode, label string, line int) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_init") {
		return nil
	}
	function := "init"
	if line == 1785 {
		function = "assert_units"
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_init", function, "src/math/subpaving/subpaving_t_def.h", line,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, label)
			bodyErr = c.displayNodeBounds(out, n)
			return bodyErr
		})
	return bodyErr
}

func (c *FixedContext) traceInitialConstraints() error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_init") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_init", "init", "src/math/subpaving/subpaving_t_def.h", 1797,
		func() bool { return true }, func(w io.Writer) error {
			bodyErr = c.DisplayConstraints(splitTraceStreamWriter{out: w}, false)
			return bodyErr
		})
	return bodyErr
}
