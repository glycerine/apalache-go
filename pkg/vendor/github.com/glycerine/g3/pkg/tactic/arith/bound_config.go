package arith

// Source: src/tactic/arith/bound_propagator.cpp:118-137 and
// src/tactic/arith/bound_propagator.h:183,260-262.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/params"
)

// UpdateParams reads the four concrete names used by the retained inequality
// tactic. The caller keeps ownership of the parameter context and value.
func (s *BoundEquationStore) UpdateParams(c *params.ParamsContext, v *params.ParamsRef) error {
	if err := s.open(); err != nil {
		return err
	}
	if c == nil || v == nil {
		return arena.ErrInvalidSlot
	}
	maxRefinements := "bound_max_refinements"
	threshold := "bound_threshold"
	smallInterval := "bound_small_interval"
	strict2double := "strict2double"
	var err error
	if s.maxRefinements, err = c.GetUIntName(v, &maxRefinements, 16); err != nil {
		return err
	}
	if s.threshold, err = c.GetDoubleName(v, &threshold, 0.05); err != nil {
		return err
	}
	if s.smallInterval, err = c.GetDoubleName(v, &smallInterval, 128); err != nil {
		return err
	}
	s.strict2double, err = c.GetDoubleName(v, &strict2double, 0.00001)
	return err
}

func (s *BoundEquationStore) HasLower(x uint32) (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	if uint64(x) >= uint64(len(s.variables)) {
		return false, arena.ErrBounds
	}
	return s.variables[x].lower != nil, nil
}

func (s *BoundEquationStore) HasUpper(x uint32) (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	if uint64(x) >= uint64(len(s.variables)) {
		return false, arena.ErrBounds
	}
	return s.variables[x].upper != nil, nil
}

func (s *BoundEquationStore) Inconsistent() (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	return s.conflict != noEquationPosition, nil
}

// PropagationCounts are the two progress counters read by the retained tactic.
func (s *BoundEquationStore) PropagationCounts() (propagations, falseAlarms uint32, err error) {
	if err = s.open(); err != nil {
		return 0, 0, err
	}
	return s.propagations, s.falseAlarms, nil
}
