package euclid

// Source: src/math/euclid/euclidean_solver.cpp:734-740,751-756; src/util/trace.h.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *solverSetup) traceNormalizeRow(line int, heading string, as []numeral.Int, xs []uint32) error {
	tags := s.traceTags()
	if !tags.ShouldTrace("euclidean_solver") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("euclidean_solver", "normalize", "src/math/euclid/euclidean_solver.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, heading)
			if err := s.writeTraceIntRow(w, as, xs); err != nil {
				formatErr = err
				return err
			}
			_, _ = io.WriteString(w, "\n")
			return nil
		})
	return formatErr
}

func (s *solverSetup) traceTags() *diagnostic.TraceTags {
	if s.trace != nil {
		return s.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

// writeTraceIntRow evaluates each source numeral conversion even if the
// borrowed ostream has already refused an insertion.
func (s *solverSetup) writeTraceIntRow(w io.Writer, as []numeral.Int, xs []uint32) error {
	for i := range as {
		value, err := s.manager.Integers().String(&as[i])
		if err != nil {
			return err
		}
		_, _ = io.WriteString(w, value)
		_, _ = io.WriteString(w, "*x")
		_, _ = io.WriteString(w, strconv.FormatUint(uint64(xs[i]), 10))
		_, _ = io.WriteString(w, " ")
	}
	return nil
}
