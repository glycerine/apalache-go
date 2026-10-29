package euclid

// Source: src/math/euclid/euclidean_solver.cpp:436-444; src/util/trace.h.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *solverSetup) writeTraceEquation(w io.Writer, eq *equation) error {
	for i, id := range eq.js {
		if i != 0 {
			_, _ = io.WriteString(w, " ")
		}
		value, err := s.manager.String(&eq.bs[i])
		if err != nil {
			return err
		}
		_, _ = io.WriteString(w, value)
		_, _ = io.WriteString(w, "*j")
		_, _ = io.WriteString(w, strconv.FormatUint(uint64(id), 10))
	}
	if len(eq.js) != 0 {
		_, _ = io.WriteString(w, " ")
	}
	_, _ = io.WriteString(w, "|= ")
	for i, id := range eq.xs {
		value, err := s.manager.Integers().String(&eq.as[i])
		if err != nil {
			return err
		}
		_, _ = io.WriteString(w, value)
		_, _ = io.WriteString(w, "*x")
		_, _ = io.WriteString(w, strconv.FormatUint(uint64(id), 10))
		_, _ = io.WriteString(w, " + ")
	}
	value, err := s.manager.Integers().String(&eq.c)
	if err != nil {
		return err
	}
	_, _ = io.WriteString(w, value)
	_, _ = io.WriteString(w, " = 0")
	return nil
}

func (s *solverSetup) traceApplyBefore(a1 *numeral.Int, sol, target *equation) error {
	tags := s.traceTags()
	if !tags.ShouldTrace("euclidean_solver_apply") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("euclidean_solver_apply", "apply_solution", "src/math/euclid/euclidean_solver.cpp", 436,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "applying: ")
			value, err := s.manager.Integers().String(a1)
			if err != nil {
				formatErr = err
				return err
			}
			_, _ = io.WriteString(w, value)
			_, _ = io.WriteString(w, " * ")
			if err = s.writeTraceEquation(w, sol); err != nil {
				formatErr = err
				return err
			}
			_, _ = io.WriteString(w, "\n")
			if err = s.writeTraceIntRow(w, target.as, target.xs); err != nil {
				formatErr = err
				return err
			}
			_, _ = io.WriteString(w, "\n")
			return nil
		})
	return formatErr
}

func (s *solverSetup) traceApplyAfter(target *equation) error {
	tags := s.traceTags()
	if !tags.ShouldTrace("euclidean_solver_apply") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("euclidean_solver_apply", "apply_solution", "src/math/euclid/euclidean_solver.cpp", 444,
		func() bool { return true }, func(w io.Writer) error {
			if err := s.writeTraceIntRow(w, target.as, target.xs); err != nil {
				formatErr = err
				return err
			}
			_, _ = io.WriteString(w, "\n")
			return nil
		})
	return formatErr
}
