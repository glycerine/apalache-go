package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:516-523,1809-1810;
// concrete config_mpfx search parameter display and TRACE caller.

import (
	"io"
	"strconv"
)

// DisplayParams writes the five source search settings in their original
// order. It reads the context's live numeral values after parameter updates.
func (c *FixedContext) DisplayParams(out io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	if err := fixedWrite(out, "max_nodes  "+strconv.FormatUint(uint64(c.maxNodes), 10)+"\n"); err != nil {
		return err
	}
	if err := fixedWrite(out, "max_depth  "+strconv.FormatUint(uint64(c.maxDepth), 10)+"\n"); err != nil {
		return err
	}
	if err := fixedWrite(out, "epsilon    "); err != nil {
		return err
	}
	epsilon, err := c.numbers.ToRationalString(c.epsilon)
	if err != nil {
		return err
	}
	if err := fixedWrite(out, epsilon+"\n"); err != nil {
		return err
	}
	if err := fixedWrite(out, "max_bound  "); err != nil {
		return err
	}
	bound, err := c.numbers.ToRationalString(c.maxBound)
	if err != nil {
		return err
	}
	if err := fixedWrite(out, bound+"\n"); err != nil {
		return err
	}
	return fixedWrite(out, "max_memory "+strconv.FormatUint(c.maxMemory, 10)+"\n")
}

func (c *FixedContext) traceSearchParams() error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_main") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_main", "operator()", "src/math/subpaving/subpaving_t_def.h", 1810,
		func() bool { return true }, func(w io.Writer) error {
			bodyErr = c.DisplayParams(splitTraceStreamWriter{out: w})
			return bodyErr
		})
	return bodyErr
}
