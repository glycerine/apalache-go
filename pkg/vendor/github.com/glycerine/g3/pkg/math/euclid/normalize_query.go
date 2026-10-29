package euclid

// Source: src/math/euclid/euclidean_solver.cpp:735-783.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// normalizeQuery applies the current solution set to a borrowed polynomial.
// The returned magnitude is the gcd of the remaining variable coefficients;
// js identifies the resulting justification combination, whose rational
// coefficients are temporary in the source and likewise not returned here.
func (s *solverSetup) normalizeQuery(as []numeral.Int, xs []uint32, c,
	aPrime, cPrime *numeral.Int, js *[]uint32) (err error) {
	if err = s.open(); err != nil {
		return err
	}
	if len(as) != len(xs) || c == nil || aPrime == nil || cPrime == nil || js == nil {
		return arena.ErrInvalidSlot
	}
	if err = s.traceNormalizeRow(734, "before applying solution set\n", as, xs); err != nil {
		return err
	}
	z := s.manager.Integers()
	row := &equation{manager: s.manager, as: make([]numeral.Int, len(as)),
		xs: append([]uint32(nil), xs...)}
	s.normRow = row
	defer func() {
		if err != nil {
			s.stop = err
			return
		}
		if cleanup := row.Close(); cleanup != nil {
			err = cleanup
			s.stop = cleanup
			return
		}
		s.normRow = nil
	}()
	for i := range as {
		if err = z.Copy(&row.as[i], &as[i]); err != nil {
			return err
		}
	}
	if err = sortInt(row.as, row.xs, s.asBuffer); err != nil {
		return err
	}
	*js = (*js)[:0]
	if err = z.Copy(cPrime, c); err != nil {
		return err
	}
	if err = s.applySolutionToEquationWithConstant(row, cPrime); err != nil {
		return err
	}
	if err = s.traceNormalizeRow(751, "after applying solution set\n", row.as, row.xs); err != nil {
		return err
	}
	*js = append(*js, row.js...)
	if len(row.as) == 0 {
		return z.SetInt32(aPrime, 0)
	}
	if err = z.Copy(aPrime, &row.as[0]); err != nil {
		return err
	}
	if err = z.Abs(aPrime); err != nil {
		return err
	}
	a := numeral.NewInt(0)
	defer func() { err = errors.Join(err, z.Del(&a)) }()
	for i := 1; i < len(row.as); i++ {
		one, e := z.IsOne(aPrime)
		if e != nil {
			return e
		}
		if one {
			break
		}
		if err = z.Copy(&a, &row.as[i]); err != nil {
			return err
		}
		if err = z.Abs(&a); err != nil {
			return err
		}
		if err = z.GCD(aPrime, &a, aPrime); err != nil {
			return err
		}
	}
	return nil
}
