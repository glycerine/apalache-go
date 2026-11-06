package rel

import (
	"fmt"
	"io"
)

func (b *AAGBuilder) Write(out io.Writer) error {
	if e := b.open(); e != nil {
		return e
	}
	if out == nil {
		return ErrAAG
	}
	maximum := (b.next - 2) / 2
	if _, e := fmt.Fprintf(out, "aag %d %d %d %d %d\n", maximum, len(b.inputs), len(b.latches), len(b.outputs), len(b.gateOrder)); e != nil {
		return e
	}
	for _, id := range b.inputs {
		if _, e := fmt.Fprintln(out, id); e != nil {
			return e
		}
	}
	for _, l := range b.latches {
		if _, e := fmt.Fprintf(out, "%d %d\n", l.current, l.next); e != nil {
			return e
		}
	}
	for _, id := range b.outputs {
		if _, e := fmt.Fprintln(out, id); e != nil {
			return e
		}
	}
	for _, g := range b.gateOrder {
		if _, e := fmt.Fprintf(out, "%d %d %d\n", g.out, g.left, g.right); e != nil {
			return e
		}
	}
	return nil
}
func (b *AAGBuilder) Counts() (inputs, latches, outputs, gates int, err error) {
	if err = b.open(); err != nil {
		return
	}
	return len(b.inputs), len(b.latches), len(b.outputs), len(b.gateOrder), nil
}
