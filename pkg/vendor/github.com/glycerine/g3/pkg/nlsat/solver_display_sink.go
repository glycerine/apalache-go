package nlsat

// Source: src/nlsat/nlsat_solver.cpp:2554-3014.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

// solverDisplaySink binds the polynomial display callback surface to the
// solver's stream. The polynomial manager borrows it only for one call.
type solverDisplaySink struct {
	out      io.Writer
	variable func(io.Writer, uint32) error
	err      error
}

func newSolverDisplaySink(out io.Writer, variable func(io.Writer, uint32) error) (*solverDisplaySink, error) {
	if out == nil {
		return nil, arena.ErrConfig
	}
	if variable == nil {
		variable = displayDefaultVariable
	}
	return &solverDisplaySink{out: out, variable: variable}, nil
}

func displayDefaultVariable(out io.Writer, variable uint32) error {
	return displayWriteString(out, "x"+strconv.FormatUint(uint64(variable), 10))
}

func displayWrite(out io.Writer, value []byte) error {
	if out == nil {
		return arena.ErrConfig
	}
	n, err := out.Write(value)
	if err == nil && n != len(value) {
		err = io.ErrShortWrite
	}
	return err
}

func displayWriteString(out io.Writer, value string) error {
	if out == nil {
		return arena.ErrConfig
	}
	n, err := io.WriteString(out, value)
	if err == nil && n != len(value) {
		err = io.ErrShortWrite
	}
	return err
}

func (s *solverDisplaySink) fail(err error) error {
	if s.err == nil {
		s.err = err
	}
	return s.err
}

func (s *solverDisplaySink) Write(value []byte) (int, error) {
	if s == nil || s.out == nil {
		return 0, arena.ErrConfig
	}
	if s.err != nil {
		return 0, s.err
	}
	n, err := s.out.Write(value)
	if err == nil && n != len(value) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return n, s.fail(err)
	}
	return n, nil
}

func (s *solverDisplaySink) WriteLiteral(value string) error {
	if s == nil || s.out == nil {
		return arena.ErrConfig
	}
	if s.err != nil {
		return s.err
	}
	if err := displayWriteString(s.out, value); err != nil {
		return s.fail(err)
	}
	return nil
}

func (s *solverDisplaySink) WriteUnsigned(value uint32) error {
	return s.WriteLiteral(strconv.FormatUint(uint64(value), 10))
}

func (s *solverDisplaySink) WriteVariable(value uint32) error {
	if s == nil || s.out == nil || s.variable == nil {
		return arena.ErrConfig
	}
	if s.err != nil {
		return s.err
	}
	if err := s.variable(s.out, value); err != nil {
		return s.fail(err)
	}
	return nil
}

func (s *solverDisplaySink) WriteDecimal(value *polynomial.PolynomialDecimalView) error {
	if s == nil || s.out == nil || value == nil {
		return arena.ErrConfig
	}
	if s.err != nil {
		return s.err
	}
	n, err := value.Length()
	if err != nil {
		return s.fail(err)
	}
	var buf [64]byte
	for offset := uint32(0); offset < n; {
		count := uint32(len(buf))
		if remaining := n - offset; remaining < count {
			count = remaining
		}
		for i := uint32(0); i < count; i++ {
			buf[i], err = value.Byte(offset + i)
			if err != nil {
				return s.fail(err)
			}
		}
		if err = displayWrite(s.out, buf[:count]); err != nil {
			return s.fail(err)
		}
		offset += count
	}
	return nil
}
