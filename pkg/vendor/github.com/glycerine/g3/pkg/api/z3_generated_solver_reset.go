package api

// Source: src/api/api_solver.cpp, persistent solver parameters and reset.

import "errors"

func (s *replaySolver) setParameters(p *ParameterSet) (err error) {
	next, err := s.owner.context.NewParameterSet()
	if err != nil {
		return err
	}
	published := false
	defer func() {
		if !published {
			err = errors.Join(err, next.Close())
		}
	}()
	if s.params != nil {
		if err = next.params.Append(&next.value, &s.params.value); err != nil {
			return err
		}
	}
	if err = next.params.Append(&next.value, &p.value); err != nil {
		return err
	}
	if err = s.value.SetParameters(p); err != nil {
		return err
	}
	old := s.params
	s.params = next
	published = true
	if old != nil {
		if err = old.Close(); err != nil {
			s.retiredParams = append(s.retiredParams, old)
			return err
		}
	}
	return nil
}

func replaySolverReset(r *Z3Replayer) (err error) {
	s, err := replayGetSolver(r)
	if err != nil {
		return err
	}
	var next *APISimpleSolver
	switch s.kind {
	case 0:
		next, err = s.owner.context.NewSolver()
	case 1:
		next, err = s.owner.context.NewSimpleSolver()
	case 2:
		next, err = s.owner.context.NewSolverForLogic(s.logic)
	case 3:
		next, err = s.owner.context.NewSolverFromTactic(s.tactic)
	default:
		return ErrZ3ReplayState
	}
	if err != nil {
		return err
	}
	published := false
	defer func() {
		if !published {
			err = errors.Join(err, next.Close())
		}
	}()
	if s.params != nil {
		if err = next.SetParameters(s.params); err != nil {
			return err
		}
	}
	if err = s.value.Close(); err != nil {
		return err
	}
	s.value = next
	published = true
	return nil
}
