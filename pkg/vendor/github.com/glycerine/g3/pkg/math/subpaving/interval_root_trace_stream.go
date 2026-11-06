package subpaving

// Source: src/util/trace.h:40-46, concrete interval-root trace operands.

import (
	"errors"
	"io"
)

// Source ostream insertion failures are sticky but do not stop later operand
// computations. Numeric formatting failures still unwind the TRACE body.
type intervalRootTraceStream struct {
	w         io.Writer
	streamErr error
}

func (s *intervalRootTraceStream) Write(p []byte) (int, error) {
	n, err := s.w.Write(p)
	if err != nil && s.streamErr == nil {
		s.streamErr = err
	}
	return n, err
}

func (s *intervalRootTraceStream) operand(err error) error {
	if err != nil && (s.streamErr == nil || !errors.Is(err, s.streamErr)) {
		return err
	}
	return nil
}
