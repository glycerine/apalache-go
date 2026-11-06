package api

import (
	"io"
	"strings"
)

// WriteFixedpointReplaySMT2 exports every registered relation, background
// assertion and normalized rule as fixedpoint SMT-LIB without a query. This
// explicit replay format is independent of the source display print options.
func (c *SMTContext) WriteFixedpointReplaySMT2(output io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	if output == nil {
		return ErrSMTContextOwner
	}
	return c.command.WriteFixedpointReplaySMT2(output)
}

// FixedpointReplaySMT2 returns the stream from WriteFixedpointReplaySMT2.
// LoadFixedpointSMT2 accepts this stream in another context.
func (c *SMTContext) FixedpointReplaySMT2() (string, error) {
	var output strings.Builder
	if err := c.WriteFixedpointReplaySMT2(&output); err != nil {
		return "", err
	}
	return output.String(), nil
}
