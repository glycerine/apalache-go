package tactic

// Source: src/math/subpaving/tactic/subpaving_tactic.cpp:266-287.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/util/params"
)

// newPublicSimplifyParams builds the two independently owned parameter sets
// used by mk_subpaving_tactic before the concrete subpaving core. The caller
// must delete both results with closePublicSimplifyParams.
func newPublicSimplifyParams(c *params.ParamsContext, source *params.ParamsRef) (first, second params.ParamsRef, err error) {
	if c == nil || source == nil {
		return first, second, params.ErrParamsOwner
	}
	first, err = c.NewValue()
	if err != nil {
		return first, second, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, c.Del(&second), c.Del(&first))
		}
	}()
	if err = c.Assign(&first, source); err != nil {
		return first, second, err
	}
	second, err = c.NewValue()
	if err != nil {
		return first, second, err
	}
	if err = c.Assign(&second, source); err != nil {
		return first, second, err
	}
	for _, setting := range []struct {
		name  string
		value bool
	}{
		{"arith_lhs", true},
		{"expand_power", true},
		{"som", true},
		{"eq2ineq", true},
		{"elim_and", true},
		{"blast_distinct", true},
	} {
		if err = c.SetBoolName(&first, &setting.name, setting.value); err != nil {
			return first, second, err
		}
	}
	name := "max_power"
	if err = c.SetUIntName(&first, &name, math.MaxUint32); err != nil {
		return first, second, err
	}
	name = "mul_to_power"
	if err = c.SetBoolName(&second, &name, true); err != nil {
		return first, second, err
	}
	return first, second, nil
}

func closePublicSimplifyParams(c *params.ParamsContext, first, second *params.ParamsRef) error {
	if c == nil || first == nil || second == nil {
		return params.ErrParamsOwner
	}
	return errors.Join(c.Del(second), c.Del(first))
}
