package bv

// Source: src/tactic/bv/bv_bounds_tactic.cpp updt_params/get_param_descrs.

import uparams "github.com/glycerine/g3/pkg/util/params"

func (s *BVBoundsSimplifier) UpdateParams(p *uparams.ParamsRef) error {
	if p == nil {
		return nil
	}
	if s.context == nil {
		return uparams.ErrParamsOwner
	}
	name := "propagate_eq"
	value, e := s.context.GetBoolName(p, &name, false)
	if e == nil {
		s.propagateEq = value
	}
	return e
}
