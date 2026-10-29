package euclid

// Source: src/math/euclid/euclidean_solver.cpp:583-605,653,699-701; src/util/trace.h.

import (
	"io"
	"strconv"
)

func (s *solverSetup) emitPivotTrace(function string, line int, body func(io.Writer) error) error {
	tags := s.traceTags()
	if !tags.ShouldTrace("euclidean_solver") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("euclidean_solver", function, "src/math/euclid/euclidean_solver.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			formatErr = body(w)
			return formatErr
		})
	return formatErr
}

func (s *solverSetup) traceMostRecentSolution(x uint32, eq *equation) error {
	return s.emitPivotTrace("substitute_most_recent_solution", 583, func(w io.Writer) error {
		_, _ = io.WriteString(w, "applying solution for x")
		_, _ = io.WriteString(w, strconv.FormatUint(uint64(x), 10))
		_, _ = io.WriteString(w, "\n")
		if err := s.writeTraceEquation(w, eq); err != nil {
			return err
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	})
}

func (s *solverSetup) traceSubstitutionEquation(x uint32, eq *equation, after bool) error {
	line, heading := 592, "eq before substituting x"
	if after {
		line, heading = 594, "eq after substituting x"
	}
	return s.emitPivotTrace("substitute_most_recent_solution", line, func(w io.Writer) error {
		_, _ = io.WriteString(w, heading)
		_, _ = io.WriteString(w, strconv.FormatUint(uint64(x), 10))
		_, _ = io.WriteString(w, "\n")
		if err := s.writeTraceEquation(w, eq); err != nil {
			return err
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	})
}

func (s *solverSetup) traceUnitElimination(eq *equation) error {
	return s.emitPivotTrace("elim_unit", 605, func(w io.Writer) error {
		_, _ = io.WriteString(w, "eliminating equation with unit coefficient:\n")
		if err := s.writeTraceEquation(w, eq); err != nil {
			return err
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	})
}

func (s *solverSetup) traceDecompositionStart(x uint32, eq *equation) error {
	return s.emitPivotTrace("decompose_and_elim", 653, func(w io.Writer) error {
		_, _ = io.WriteString(w, "decompositing equation for x")
		_, _ = io.WriteString(w, strconv.FormatUint(uint64(x), 10))
		_, _ = io.WriteString(w, "\n")
		if err := s.writeTraceEquation(w, eq); err != nil {
			return err
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	})
}

func (s *solverSetup) traceDecompositionResult(parameter uint32, auxiliary, current *equation) error {
	return s.emitPivotTrace("decompose_and_elim", 699, func(w io.Writer) error {
		_, _ = io.WriteString(w, "decomposition: new parameter x")
		_, _ = io.WriteString(w, strconv.FormatUint(uint64(parameter), 10))
		_, _ = io.WriteString(w, " aux eq:\n")
		if err := s.writeTraceEquation(w, auxiliary); err != nil {
			return err
		}
		_, _ = io.WriteString(w, "\n")
		if err := s.writeTraceEquation(w, current); err != nil {
			return err
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	})
}
