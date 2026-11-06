package euclid

// Source: src/math/euclid/euclidean_solver.cpp:173-222,504-510,547-564.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// normalizeEquation preserves the source's constant contradiction, trivial
// equality, coefficient gcd and divisibility branches. It leaves the first
// inconsistent equation owned by the todo list for its justification.
func (s *solverSetup) normalizeEquation(eqIndex uint32) (err error) {
	if err = s.open(); err != nil {
		return err
	}
	if s.inconsistent != nullEquationIndex {
		return nil
	}
	if eqIndex >= uint32(len(s.equations)) || s.equations[eqIndex] == nil {
		return arena.ErrInvalidSlot
	}
	eq := s.equations[eqIndex]
	z := s.manager.Integers()
	if err = s.traceEquationRecord(184, "normalizing:\n", eq); err != nil {
		return err
	}
	if len(eq.xs) == 0 {
		var zero bool
		if zero, err = z.IsZero(&eq.c); err != nil {
			return err
		}
		if !zero {
			s.traceNormalizationDiagnostic(189, "c = 0 inconsistency detected\n")
			s.inconsistent = eqIndex
			return nil
		}
		if err = eq.Close(); err != nil {
			return err
		}
		s.equations[eqIndex] = nil
		return nil
	}
	g, a := numeral.NewInt(0), numeral.NewInt(0)
	defer func() { err = errors.Join(err, z.Del(&g), z.Del(&a)) }()
	if err = z.Copy(&g, &eq.as[0]); err != nil {
		return err
	}
	if err = z.Abs(&g); err != nil {
		return err
	}
	for i := 1; i < len(eq.as); i++ {
		var one bool
		if one, err = z.IsOne(&g); err != nil {
			return err
		}
		if one {
			break
		}
		if err = z.Copy(&a, &eq.as[i]); err != nil {
			return err
		}
		if err = z.Abs(&a); err != nil {
			return err
		}
		if err = z.GCD(&g, &a, &g); err != nil {
			return err
		}
	}
	one, e := z.IsOne(&g)
	if e != nil {
		return e
	}
	if one {
		return nil
	}
	divides, e := z.Divides(&g, &eq.c)
	if e != nil {
		return e
	}
	if !divides {
		s.traceNormalizationDiagnostic(214, "gcd inconsistency detected\n")
		s.inconsistent = eqIndex
		return nil
	}
	for i := range eq.as {
		if err = z.Div(&eq.as[i], &g, &eq.as[i]); err != nil {
			return err
		}
	}
	for i := range eq.bs {
		if err = s.manager.DivInt(&eq.bs[i], &g, &eq.bs[i]); err != nil {
			return err
		}
	}
	if err = z.Del(&g); err != nil {
		return err
	}
	if err = z.Del(&a); err != nil {
		return err
	}
	if err = s.traceEquationRecord(222, "after normalization:\n", eq); err != nil {
		return err
	}
	return nil
}

func (s *solverSetup) addOccurrences(eqIndex uint32) error {
	if e := s.open(); e != nil {
		return e
	}
	if eqIndex >= uint32(len(s.equations)) || s.equations[eqIndex] == nil {
		return arena.ErrInvalidSlot
	}
	for _, x := range s.equations[eqIndex].xs {
		if x >= uint32(len(s.occurrences)) {
			return arena.ErrBounds
		}
		s.occurrences[x] = append(s.occurrences[x], eqIndex)
	}
	return nil
}

// assertEquation follows assert_eq's publication and processing order. The
// optional justification contributes coefficient one before substitution.
func (s *solverSetup) assertEquation(as []numeral.Int, xs []uint32, c *numeral.Int, justification uint32) (err error) {
	if err = s.open(); err != nil {
		return err
	}
	if s.inconsistent != nullEquationIndex {
		return nil
	}
	var bs []numeral.Rational
	var js []uint32
	if justification != nullEquationIndex {
		if justification >= s.nextJustification {
			return arena.ErrBounds
		}
		one := numeral.NewRational()
		defer func() { err = errors.Join(err, s.manager.Del(&one)) }()
		if err = s.manager.SetInt64(&one, 1); err != nil {
			return err
		}
		bs, js = []numeral.Rational{one}, []uint32{justification}
	}
	eq, e := makeEquation(s.manager, as, xs, c, bs, js, s.asBuffer, s.bsBuffer, true, s.trace)
	if e != nil {
		s.pendingEq = eq
		s.stop = e
		return e
	}
	s.pendingEq = eq
	if e = s.traceAssertEquationRecord(559, "new-eq:\n", eq); e != nil {
		s.stop = e
		return e
	}
	s.equations = append(s.equations, eq)
	s.pendingEq = nil
	eqIndex := uint32(len(s.equations) - 1)
	if e = s.applySolutionToEquation(eq); e != nil {
		s.stop = e
		return e
	}
	if e = s.normalizeEquation(eqIndex); e != nil {
		s.stop = e
		return e
	}
	if s.equations[eqIndex] != nil {
		if e = s.addOccurrences(eqIndex); e != nil {
			s.stop = e
			return e
		}
		if e = s.traceAssertEquationRecord(565, "asserted:\n", eq); e != nil {
			s.stop = e
			return e
		}
	}
	return nil
}
