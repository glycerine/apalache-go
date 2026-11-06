package interval

// Source: src/util/trace.h:40-46, concrete scalar-root trace operands.

import (
	"errors"
	"io"
)

type rootTraceOperandWriter struct {
	w         io.Writer
	streamErr error
}

func (s *rootTraceOperandWriter) Write(p []byte) (int, error) {
	n, err := s.w.Write(p)
	if err != nil && s.streamErr == nil {
		s.streamErr = err
	}
	return n, err
}

// A failed ostream insertion sticks while the remaining operands are still
// computed. A non-stream formatter error unwinds before the TRACE footer.
func (s *rootTraceOperandWriter) operand(err error) error {
	if err != nil && (s.streamErr == nil || !errors.Is(err, s.streamErr)) {
		return err
	}
	return nil
}
