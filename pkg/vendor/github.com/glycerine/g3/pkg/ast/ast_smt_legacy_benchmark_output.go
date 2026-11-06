package ast

// Source: ast_smt_pp::display_smt2 writes through one source ostream. A failed
// stream suppresses subsequent writes, while later printer operands still run.

import "io"

type legacySMTBenchmarkOutput struct {
	stream io.Writer
	err    error
}

func (out *legacySMTBenchmarkOutput) WriteString(text string) {
	if out.err != nil {
		return
	}
	n, err := io.WriteString(out.stream, text)
	if err == nil && n != len(text) {
		err = io.ErrShortWrite
	}
	out.err = err
}

func (out *legacySMTBenchmarkOutput) WriteByte(b byte) error {
	out.WriteString(string([]byte{b}))
	return out.err
}
