package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1210-1311; config_mpf.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (c *ArbitraryContext) traceRelevantCandidate(variable uint32, k *numeral.ArbitraryFloat, lower, open bool, currentLower, currentUpper *ArbitraryBound) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_relevant_bound") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_relevant_bound", "relevant_new_bound", "src/math/subpaving/subpaving_t_def.h", 1210,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			if err := c.writeVariable(out, variable); err != nil {
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
			for _, bound := range []*ArbitraryBound{currentLower, currentUpper} {
				if bound != nil {
					if err := c.displayBound(out, bound); err != nil {
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

func (c *ArbitraryContext) traceRelevantReason(line int, prefix string, threshold *numeral.ArbitraryFloat) error {
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

func (c *ArbitraryContext) traceRelevantDecision(relevant bool, line int, prefix string, threshold *numeral.ArbitraryFloat) (bool, error) {
	if err := c.traceRelevantReason(line, prefix, threshold); err != nil {
		return false, err
	}
	return relevant, nil
}

func (c *ArbitraryContext) traceRelevantLowerBug(k, threshold, lower, minimum *numeral.ArbitraryFloat) error {
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
				value *numeral.ArbitraryFloat
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
