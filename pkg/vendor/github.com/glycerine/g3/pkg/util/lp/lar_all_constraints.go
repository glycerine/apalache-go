package lp

// Source: src/util/lp/lar_solver.h:1054-1067.

import "os"

func (s *LARSolver) AllConstraintsHold() (holds bool, err error) {
	if err = s.ready(); err != nil {
		return false, err
	}
	if s.validationModel != nil {
		return false, ErrExactVectorPending
	}
	defer func() { s.stopped = err != nil }()
	cancel, e := s.Settings.CancelFlag()
	if e != nil {
		return false, e
	}
	if cancel {
		return true, nil
	}
	// This source lexical map is shared with the nested extraction/evaluation
	// methods. On error it remains owned until solver cleanup.
	s.validationModel = NewLARRationalModel(s.manager)
	if err = s.GetModel(s.validationModel); err != nil {
		return false, err
	}
	for i, c := range s.Constraints {
		holds, err = s.ConstraintHolds(c, s.validationModel)
		if err != nil {
			return false, err
		}
		if !holds {
			if err = s.PrintConstraint(uint32(i), os.Stdout); err != nil {
				return false, err
			}
			if err = s.validationModel.Close(); err != nil {
				return false, err
			}
			s.validationModel = nil
			return false, nil
		}
	}
	if err = s.validationModel.Close(); err != nil {
		return false, err
	}
	s.validationModel = nil
	return true, nil
}
