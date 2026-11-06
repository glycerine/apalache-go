package nlsat

// Source: src/nlsat/nlsat_solver.cpp:2977-3062.

import "errors"

func (s *Solver) fixFourierMotzkinBounds() (err error) {
	if len(s.fmBounds) == 0 {
		return nil
	}
	lo, err := NewAssignment(s.config.Algebraic)
	if err != nil {
		return err
	}
	hi, err := NewAssignment(s.config.Algebraic)
	if err != nil {
		_ = lo.Close()
		return err
	}
	defer func() { err = errors.Join(err, hi.Close(), lo.Close()) }()
	for _, bound := range s.fmBounds {
		if err = s.assignment.ResetVar(bound.variable); err != nil {
			return err
		}
	}
	for i := len(s.fmBounds) - 1; i >= 0; i-- {
		if err = s.fixFourierMotzkinBound(s.fmBounds[i], lo, hi); err != nil {
			return err
		}
	}
	return nil
}
