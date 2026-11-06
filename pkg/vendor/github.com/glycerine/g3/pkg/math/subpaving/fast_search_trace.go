package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:516-522,1810-1838; config_mpff.

import (
	"io"
	"strconv"
)

func (c *FastContext) traceSearchParams() error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_main") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_main", "operator()", "src/math/subpaving/subpaving_t_def.h", 1810,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, "max_nodes  ")
			_, _ = io.WriteString(out, strconv.FormatUint(uint64(c.maxNodes), 10))
			_, _ = io.WriteString(out, "\nmax_depth  ")
			_, _ = io.WriteString(out, strconv.FormatUint(uint64(c.maxDepth), 10))
			_, _ = io.WriteString(out, "\nepsilon    ")
			epsilon, err := c.numbers.String(c.epsilon)
			if err != nil {
				bodyErr = err
				return err
			}
			_, _ = io.WriteString(out, epsilon)
			_, _ = io.WriteString(out, "\nmax_bound  ")
			bound, err := c.numbers.String(c.maxBound)
			if err != nil {
				bodyErr = err
				return err
			}
			_, _ = io.WriteString(out, bound)
			_, _ = io.WriteString(out, "\nmax_memory ")
			_, _ = io.WriteString(out, strconv.FormatUint(c.maxMemory, 10))
			_, _ = io.WriteString(out, "\n")
			return nil
		})
	return bodyErr
}

func (c *FastContext) traceSearchNode(n *FastNode, phase string, line int) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_main") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_main", "operator()", "src/math/subpaving/subpaving_t_def.h", line,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			id := strconv.FormatUint(uint64(n.id), 10)
			switch phase {
			case "selected":
				_, _ = io.WriteString(out, "selected node: #")
				_, _ = io.WriteString(out, id)
				_, _ = io.WriteString(out, ", depth: ")
				_, _ = io.WriteString(out, strconv.FormatUint(uint64(n.depth), 10))
				_, _ = io.WriteString(out, "\n")
			case "after":
				_, _ = io.WriteString(out, "node #")
				_, _ = io.WriteString(out, id)
				_, _ = io.WriteString(out, " after propagation\n")
				if err := c.displayNodeBounds(out, n); err != nil {
					bodyErr = err
					return err
				}
			case "inconsistent":
				_, _ = io.WriteString(out, "node #")
				_, _ = io.WriteString(out, id)
				_, _ = io.WriteString(out, " is inconsistent.\n")
			case "depth":
				_, _ = io.WriteString(out, "maximum depth reached, skipping node #")
				_, _ = io.WriteString(out, id)
				_, _ = io.WriteString(out, "\n")
			}
			return nil
		})
	return bodyErr
}

func (c *FastContext) traceSearchVariable(variable uint32, found bool) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_main") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_main", "operator()", "src/math/subpaving/subpaving_t_def.h", 1837,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, "splitting variable: ")
			if found {
				if err := c.writeVariable(out, variable); err != nil {
					bodyErr = err
					return err
				}
			} else {
				_, _ = io.WriteString(out, "[null]")
			}
			_, _ = io.WriteString(out, "\n")
			return nil
		})
	return bodyErr
}
