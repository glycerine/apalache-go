package nlsat

// Source: src/nlsat/nlsat_solver.cpp:115,2977-2993,4147-4149.

import "errors"

func (s *Solver) closeFourierMotzkinBounds() (err error) {
	for i := len(s.fmBounds) - 1; i >= 0; i-- {
		err = errors.Join(err, s.fmBounds[i].close())
		if err != nil {
			return err
		}
	}
	s.fmBounds = nil
	return nil
}
