package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1210-1311; config_hwf.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (c *HardwareContext) traceRelevantCandidate(variable uint32, k *numeral.HardwareFloat, lower, open bool, currentLower, currentUpper *HardwareBound) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_relevant_bound") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_relevant_bound", "relevant_new_bound", "src/math/subpaving/subpaving_t_def.h", 1210,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			if err := hardwareWriteVariable(out, variable, c.displayVariable); err != nil {
				bodyErr = err
				return err
			}
			relation := " <"
			if lower {
				relation = " >"
			}
			if !open {
				relation += "="
			}
			_, _ = io.WriteString(out, relation+" ")
			if err := c.numbers.DisplayTo(out, k); err != nil {
				bodyErr = err
				return err
			}
			_, _ = io.WriteString(out, "\nexisting bounds:\n")
			for _, bound := range []*HardwareBound{currentLower, currentUpper} {
				if bound != nil {
					if err := c.displayInequality(out, bound.variable, &bound.value, bound.lower, bound.open, c.displayVariable); err != nil {
						bodyErr = err
						return err
					}
					_, _ = io.WriteString(out, "\n")
				}
			}
			return nil
		})
	return bodyErr
}

func (c *HardwareContext) traceRelevantReason(line int, prefix string, threshold *numeral.HardwareFloat) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_relevant_bound") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_relevant_bound", "relevant_new_bound", "src/math/subpaving/subpaving_t_def.h", line,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, prefix)
			if threshold != nil {
				bodyErr = c.numbers.DisplayTo(out, threshold)
				if bodyErr != nil {
					return bodyErr
				}
			}
			_, _ = io.WriteString(out, "\n")
			return nil
		})
	return bodyErr
}

func (c *HardwareContext) traceRelevantDecision(relevant bool, line int, prefix string, threshold *numeral.HardwareFloat) (bool, error) {
	if err := c.traceRelevantReason(line, prefix, threshold); err != nil {
		return false, err
	}
	return relevant, nil
}

func (c *HardwareContext) traceRelevantLowerBug(k, threshold, lower, minimum *numeral.HardwareFloat) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_relevant_bound_bug") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_relevant_bound_bug", "relevant_new_bound", "src/math/subpaving/subpaving_t_def.h", 1253,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			for i, part := range []struct {
				label string
				value *numeral.HardwareFloat
			}{{"k: ", k}, {", delta: ", threshold}, {"\ncurr_lower: ", lower}, {", min: ", minimum}} {
				_, _ = io.WriteString(out, part.label)
				if err := c.numbers.DisplayTo(out, part.value); err != nil {
					bodyErr = err
					return err
				}
				if i == 3 {
					_, _ = io.WriteString(out, "\n")
				}
			}
			return nil
		})
	return bodyErr
}
