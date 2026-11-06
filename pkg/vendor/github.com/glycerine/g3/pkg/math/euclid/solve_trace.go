package euclid

// Source: src/math/euclid/euclidean_solver.cpp:711-720; src/util/trace.h.

import (
	"io"
	"strconv"
)

func (s *solverSetup) writeTraceSolver(w io.Writer) error {
	if s.inconsistent != nullEquationIndex {
		_, _ = io.WriteString(w, "inconsistent: ")
		if err := s.writeTraceEquation(w, s.equations[s.inconsistent]); err != nil {
			return err
		}
		_, _ = io.WriteString(w, "\n")
	}
	_, _ = io.WriteString(w, "solution set:\n")
	for _, eq := range s.solution {
		if eq == nil {
			continue
		}
		if err := s.writeTraceEquation(w, eq); err != nil {
			return err
		}
		_, _ = io.WriteString(w, "\n")
	}
	_, _ = io.WriteString(w, "todo:\n")
	for _, eq := range s.equations {
		if eq == nil {
			continue
		}
		if err := s.writeTraceEquation(w, eq); err != nil {
			return err
		}
		_, _ = io.WriteString(w, "\n")
	}
	return nil
}

func (s *solverSetup) traceSolveEntry() error {
	return s.emitPivotTrace("solve", 711, func(w io.Writer) error {
		_, _ = io.WriteString(w, "solving...\n")
		return s.writeTraceSolver(w)
	})
}

func (s *solverSetup) traceSolveCandidate() error {
	return s.emitPivotTrace("solve", 714, func(w io.Writer) error {
		_, _ = io.WriteString(w, "eliminating x")
		_, _ = io.WriteString(w, strconv.FormatUint(uint64(s.nextX), 10))
		_, _ = io.WriteString(w, "\n")
		return nil
	})
}

func (s *solverSetup) traceSolveAfter() error {
	return s.emitPivotTrace("solve", 719, s.writeTraceSolver)
}
