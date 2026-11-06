package euclid

// Source: src/math/euclid/euclidean_solver.cpp:404-405; src/util/trace.h.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func writeAddMulTraceIndices(w io.Writer, i1, i2 int) {
	_, _ = io.WriteString(w, "i1: ")
	_, _ = io.WriteString(w, strconv.Itoa(i1))
	_, _ = io.WriteString(w, ", i2: ")
	_, _ = io.WriteString(w, strconv.Itoa(i2))
	_, _ = io.WriteString(w, " new_a: ")
}

func (s *solverSetup) traceIntAddMul(i1, i2 int, result, first, k, second *numeral.Int) error {
	tags := s.traceTags()
	if !tags.ShouldTrace("euclidean_solver_add_mul") {
		return nil
	}
	z := s.manager.Integers()
	var formatErr error
	_ = tags.EmitConditional("euclidean_solver_add_mul", "addmul", "src/math/euclid/euclidean_solver.cpp", 404,
		func() bool { return true }, func(w io.Writer) error {
			writeAddMulTraceIndices(w, i1, i2)
			for i, item := range [...]*numeral.Int{result, first, k, second} {
				value, err := z.String(item)
				if err != nil {
					formatErr = err
					return err
				}
				_, _ = io.WriteString(w, value)
				switch i {
				case 0:
					_, _ = io.WriteString(w, "\nas1: ")
				case 1:
					_, _ = io.WriteString(w, ", k: ")
				case 2:
					_, _ = io.WriteString(w, ", as2: ")
				default:
					_, _ = io.WriteString(w, "\n")
				}
			}
			return nil
		})
	return formatErr
}

func (s *solverSetup) traceRationalAddMul(i1, i2 int, result, first *numeral.Rational, k *numeral.Int, second *numeral.Rational) error {
	tags := s.traceTags()
	if !tags.ShouldTrace("euclidean_solver_add_mul") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("euclidean_solver_add_mul", "addmul", "src/math/euclid/euclidean_solver.cpp", 404,
		func() bool { return true }, func(w io.Writer) error {
			writeAddMulTraceIndices(w, i1, i2)
			value, err := s.manager.String(result)
			if err != nil {
				formatErr = err
				return err
			}
			_, _ = io.WriteString(w, value)
			_, _ = io.WriteString(w, "\nas1: ")
			value, err = s.manager.String(first)
			if err != nil {
				formatErr = err
				return err
			}
			_, _ = io.WriteString(w, value)
			_, _ = io.WriteString(w, ", k: ")
			value, err = s.manager.Integers().String(k)
			if err != nil {
				formatErr = err
				return err
			}
			_, _ = io.WriteString(w, value)
			_, _ = io.WriteString(w, ", as2: ")
			value, err = s.manager.String(second)
			if err != nil {
				formatErr = err
				return err
			}
			_, _ = io.WriteString(w, value)
			_, _ = io.WriteString(w, "\n")
			return nil
		})
	return formatErr
}
