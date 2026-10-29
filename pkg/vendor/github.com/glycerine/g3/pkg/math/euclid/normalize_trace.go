package euclid

// Source: src/math/euclid/euclidean_solver.cpp:184-222,559-565; src/util/trace.h.

import "io"

func (s *solverSetup) traceEquationRecord(line int, heading string, eq *equation) error {
	tags := s.traceTags()
	if !tags.ShouldTrace("euclidean_solver") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("euclidean_solver", "normalize_eq", "src/math/euclid/euclidean_solver.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, heading)
			if err := s.writeTraceEquation(w, eq); err != nil {
				formatErr = err
				return err
			}
			_, _ = io.WriteString(w, "\n")
			return nil
		})
	return formatErr
}

func (s *solverSetup) traceAssertEquationRecord(line int, heading string, eq *equation) error {
	tags := s.traceTags()
	if !tags.ShouldTrace("euclidean_solver") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("euclidean_solver", "assert_eq", "src/math/euclid/euclidean_solver.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, heading)
			if err := s.writeTraceEquation(w, eq); err != nil {
				formatErr = err
				return err
			}
			_, _ = io.WriteString(w, "\n")
			return nil
		})
	return formatErr
}

func (s *solverSetup) traceNormalizationDiagnostic(line int, message string) {
	tags := s.traceTags()
	if !tags.ShouldTrace("euclidean_solver") {
		return
	}
	_ = tags.EmitConditional("euclidean_solver", "normalize_eq", "src/math/euclid/euclidean_solver.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, message)
			return nil
		})
}
