package arith

// Source: src/tactic/arith/nla2bv_tactic.cpp:421-423,69-80;
// src/tactic/arith/bv2int_rewriter.cpp update_params.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func (t *NLA2BVTactic) Translate(m *ast.ASTManager) (result tactic.Tactic, err error) {
	var created *NLA2BVTactic
	defer func() {
		if err != nil {
			if created != nil {
				err = errors.Join(err, created.Close())
			}
			result = nil
		}
	}()
	if err = t.open(); err != nil {
		return nil, err
	}
	services, err := arithmeticTargetServices(m, t.services)
	if err != nil {
		return nil, err
	}
	c, g := services.Context, services.Globals
	p := &t.params
	if c != t.context {
		local, e := c.NewValue()
		if e != nil {
			return nil, e
		}
		defer func() { err = errors.Join(err, c.Del(&local)) }()
		// Copy exactly the policy consumed by this tactic and its BV2Int context.
		for _, field := range []struct {
			name     string
			fallback uint32
		}{
			{"nla2bv_root", 2}, {"nla2bv_divisor", 2}, {"nla2bv_max_bv_size", math.MaxUint32}, {"nla2bv_bv_size", 4}, {"max_bv_size", math.MaxUint32},
		} {
			name := field.name
			value, e := t.context.GetUIntName(&t.params, &name, field.fallback)
			if e != nil {
				return nil, e
			}
			if e = c.SetUIntName(&local, &name, value); e != nil {
				return nil, e
			}
		}
		p = &local
	}
	created, err = NewNLA2BVTactic(m, c, g, p, services)
	if err != nil {
		return nil, err
	}
	created.trace = t.trace
	return created, nil
}
