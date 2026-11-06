package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1809,1844; config_mpq.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/statistics"
)

// The source constructs a fresh statistics value inside each TRACE body,
// immediately before and after the search loop.
func (c *RationalContext) traceSearchStatistics(line int) error {
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
