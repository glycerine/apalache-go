package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1809-1844; config_mpfx.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/statistics"
)

func (c *FixedContext) traceSearchStatistics(line int) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_stats") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_stats", "operator()", "src/math/subpaving/subpaving_t_def.h", line,
		func() bool { return true }, func(w io.Writer) error {
			var st statistics.Statistics
			if err := c.CollectStatistics(&st); err != nil {
				bodyErr = err
				return err
			}
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, "statistics:\n")
			bodyErr = st.DisplaySMT2(out)
			return bodyErr
		})
	return bodyErr
}

func (c *FixedContext) traceSearchNode(n *FixedNode, phase string, line int) error {
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
				_, _ = io.WriteString(out, "selected node: #"+id+", depth: "+strconv.FormatUint(uint64(fixedNodeDepth(n)), 10)+"\n")
			case "after":
				_, _ = io.WriteString(out, "node #"+id+" after propagation\n")
				bodyErr = c.displayNodeBounds(out, n)
			case "inconsistent":
				_, _ = io.WriteString(out, "node #"+id+" is inconsistent.\n")
			case "depth":
				_, _ = io.WriteString(out, "maximum depth reached, skipping node #"+id+"\n")
			}
			return bodyErr
		})
	return bodyErr
}

func (c *FixedContext) traceSearchVariable(variable uint32, found bool) error {
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
				bodyErr = c.writeVariable(out, variable)
			} else {
				_, _ = io.WriteString(out, "[null]")
			}
			_, _ = io.WriteString(out, "\n")
			return bodyErr
		})
	return bodyErr
}
