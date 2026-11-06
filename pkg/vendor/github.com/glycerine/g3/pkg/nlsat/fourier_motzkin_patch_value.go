package nlsat

// Source: src/nlsat/nlsat_solver.cpp:2994-3062.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/polynomial"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (s *Solver) fixFourierMotzkinBound(bound *fourierMotzkinBound, lo, hi *Assignment) (err error) {
	a, b, err := bound.values()
	if err != nil {
		return err
	}
	am := s.config.Algebraic
	var av, bv, value, updated polynomial.Algebraic
	defer func() { err = errors.Join(err, am.Del(&updated), am.Del(&value), am.Del(&bv), am.Del(&av)) }()
	if err = s.config.Polynomial.EvaluateAlgebraic(a, am.AlgebraicRootCore, s.assignment, &av); err != nil {
		return err
	}
	if err = s.config.Polynomial.EvaluateAlgebraic(b, am.AlgebraicRootCore, s.assignment, &bv); err != nil {
		return err
	}
	if err = am.Neg(&bv); err != nil {
		return err
	}
	if err = am.Div(&bv, &av, &value); err != nil {
		return err
	}
	isLower, err := am.IsPos(&av)
	if err != nil {
		return err
	}
	assigned, err := s.assignment.Contains(bound.variable)
	if err != nil {
		return err
	}
	if !assigned {
		if !bound.strict {
			err = s.assignment.Set(bound.variable, &value)
		} else {
			delta := int32(-1)
			if isLower {
				delta = 1
			}
			if err = am.AddInteger(&value, ptrInteger(delta), &updated); err == nil {
				err = s.assignment.Set(bound.variable, &updated)
			}
		}
	} else {
		current, cause := s.assignment.Value(bound.variable)
		if cause != nil {
			return cause
		}
		order, cause := am.Compare(&value, current)
		if cause != nil {
			return cause
		}
		if isLower {
			satisfied := (bound.strict && order < 0) || (!bound.strict && order <= 0)
			if !satisfied {
				err = s.repairFourierMotzkinValue(bound.variable, &value, bound.strict, 1, hi, &updated)
			}
		} else {
			satisfied := (bound.strict && order > 0) || (!bound.strict && order >= 0)
			if !satisfied {
				err = s.repairFourierMotzkinValue(bound.variable, &value, bound.strict, -1, lo, &updated)
			}
		}
	}
	if err != nil {
		return err
	}
	target := hi
	better := func(order int) bool { return order > 0 }
	if isLower {
		target = lo
		better = func(order int) bool { return order < 0 }
	}
	present, err := target.Contains(bound.variable)
	if err != nil {
		return err
	}
	if !present {
		return target.Set(bound.variable, &value)
	}
	old, err := target.Value(bound.variable)
	if err != nil {
		return err
	}
	order, err := am.Compare(old, &value)
	if err != nil {
		return err
	}
	if better(order) {
		return target.Set(bound.variable, &value)
	}
	return nil
}

func ptrInteger(value int32) *unumeral.Int { v := unumeral.NewInt(value); return &v }

func (s *Solver) repairFourierMotzkinValue(variable uint32, boundary *polynomial.Algebraic, strict bool, delta int32, opposite *Assignment, updated *polynomial.Algebraic) (err error) {
	am := s.config.Algebraic
	if !strict {
		return s.assignment.Set(variable, boundary)
	}
	present, err := opposite.Contains(variable)
	if err != nil {
		return err
	}
	if !present {
		if err = am.AddInteger(boundary, ptrInteger(delta), updated); err != nil {
			return err
		}
		return s.assignment.Set(variable, updated)
	}
	other, err := opposite.Value(variable)
	if err != nil {
		return err
	}
	var sum, two polynomial.Algebraic
	defer func() { err = errors.Join(err, am.Del(&two), am.Del(&sum)) }()
	if err = am.Add(other, boundary, &sum); err != nil {
		return err
	}
	if err = am.SetInteger(&two, ptrInteger(2)); err != nil {
		return err
	}
	if err = am.Div(&sum, &two, updated); err != nil {
		return err
	}
	return s.assignment.Set(variable, updated)
}
