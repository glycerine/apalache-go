package pdr

// Source: pdr_context.cpp context::display.
import "io"

func (c *Context) WriteDisplay(out io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	for _, declaration := range c.order {
		if err := c.transformers[declaration].WriteDisplay(out); err != nil {
			return err
		}
	}
	return c.search.WriteDisplay(out)
}
