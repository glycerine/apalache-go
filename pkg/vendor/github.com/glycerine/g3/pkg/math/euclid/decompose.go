package euclid

// Source: src/math/euclid/euclidean_solver.cpp:624-705.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// decomposeScalar is the concrete mpz division and signed-remainder step for
// one coefficient or the constant. remainder may alias input, as it does in
// the source's in-place equation compaction.
func (s *solverSetup) decomposeScalar(positivePivot bool, absolutePivot, input,
	quotient, remainder *numeral.Int) (err error) {
	z := s.manager.Integers()
	positiveInput, e := z.IsPos(input)
	if e != nil {
		return e
	}
	absInput := numeral.NewInt(0)
	defer func() { err = errors.Join(err, z.Del(&absInput)) }()
	if err = z.Copy(&absInput, input); err != nil {
		return err
	}
	if !positiveInput {
		if err = z.Neg(&absInput); err != nil {
			return err
		}
	}
	positiveQuotient := positiveInput
	if positivePivot {
		positiveQuotient = !positiveQuotient
	}
	if err = z.Div(&absInput, absolutePivot, quotient); err != nil {
		return err
	}
	divides, e := z.Divides(absolutePivot, input)
	if e != nil {
		return e
	}
	if divides {
		if err = z.Reset(remainder); err != nil {
			return err
		}
	} else if positiveInput {
		if err = z.SubMul(input, absolutePivot, quotient, remainder); err != nil {
			return err
		}
	} else {
		if err = z.AddMul(input, absolutePivot, quotient, remainder); err != nil {
			return err
		}
	}
	if !positiveQuotient {
		return z.Neg(quotient)
	}
	return nil
}

// decomposeAndEliminate creates the source parameter and auxiliary solution
// equation, then substitutes that solution into remaining occurrences. The
// original equation stays in the todo list with its compacted remainders.
func (s *solverSetup) decomposeAndEliminate() (err error) {
	if err = s.open(); err != nil {
		return err
	}
	if s.nextX == nullEquationIndex || s.nextEq >= uint32(len(s.equations)) || s.equations[s.nextEq] == nil {
		return arena.ErrInvalidSlot
	}
	z := s.manager.Integers()
	one, e := z.IsOne(&s.nextA)
	if e != nil {
		return e
	}
	if one {
		return arena.ErrInvalidSlot
	}
	s.tmpXs = s.tmpXs[:0]
	if err = s.decomposeBuffer.Reset(); err != nil {
		return err
	}
	parameter, e := s.mkVar(true)
	if e != nil {
		return e
	}
	newA, newC := numeral.NewInt(0), numeral.NewInt(0)
	defer func() { err = errors.Join(err, z.Del(&newA), z.Del(&newC)) }()
	eq := s.equations[s.nextEq]
	if err = s.traceDecompositionStart(s.nextX, eq); err != nil {
		return err
	}
	size, j := len(eq.xs), 0
	for i := 0; i < size; i++ {
		x := eq.xs[i]
		if x == s.nextX {
			if err = z.SetInt32(&newA, -1); err != nil {
				return err
			}
			if err = s.decomposeBuffer.Push(&newA); err != nil {
				return err
			}
			s.tmpXs = append(s.tmpXs, x)
			if err = s.eraseOccurrence(x, s.nextEq); err != nil {
				return err
			}
			continue
		}
		if err = s.decomposeScalar(s.nextPosA, &s.nextA, &eq.as[i], &newA, &eq.as[j]); err != nil {
			return err
		}
		if err = s.decomposeBuffer.Push(&newA); err != nil {
			return err
		}
		s.tmpXs = append(s.tmpXs, x)
		zero, e := z.IsZero(&eq.as[j])
		if e != nil {
			return e
		}
		if zero {
			if err = s.eraseOccurrence(x, s.nextEq); err != nil {
				return err
			}
		} else {
			eq.xs[j] = x
			j++
		}
	}
	if err = euclidSourceAssertion(677, "j < sz", j < size); err != nil {
		return err
	}
	if j >= size {
		return arena.ErrInvalidSlot
	}
	if err = z.SetInt32(&newA, 1); err != nil {
		return err
	}
	if err = s.decomposeBuffer.Push(&newA); err != nil {
		return err
	}
	s.tmpXs = append(s.tmpXs, parameter)
	if err = z.Copy(&eq.as[j], &s.nextA); err != nil {
		return err
	}
	if !s.nextPosA {
		if err = z.Neg(&eq.as[j]); err != nil {
			return err
		}
	}
	eq.xs[j] = parameter
	j++
	for i := j; i < size; i++ {
		if err = z.Del(&eq.as[i]); err != nil {
			return err
		}
	}
	eq.as, eq.xs = eq.as[:j], eq.xs[:j]
	if err = s.decomposeScalar(s.nextPosA, &s.nextA, &eq.c, &newC, &eq.c); err != nil {
		return err
	}
	coefficients, e := s.decomposeBuffer.BorrowedValues()
	if e != nil {
		return e
	}
	aux, e := makeEquation(s.manager, coefficients, s.tmpXs, &newC, nil, nil,
		s.asBuffer, s.bsBuffer, false, s.trace)
	if e != nil {
		s.pendingEq = aux
		return e
	}
	s.pendingEq = aux
	if err = s.traceDecompositionResult(parameter, aux, eq); err != nil {
		return err
	}
	s.solved[s.nextX] = uint32(len(s.solution))
	s.solution = append(s.solution, aux)
	s.pendingEq = nil
	return s.substituteMostRecentSolution(s.nextX)
}
