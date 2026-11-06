package qe

// Source: src/qe/qe_sat_tactic.cpp, tactic parameters.

import (
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type satTacticOptions struct {
	extrapolate                               uint32
	projection, strongSimplify, simplifyLocal bool
}

func newSATTacticOptions() satTacticOptions {
	return satTacticOptions{projection: true, strongSimplify: true}
}

// update preserves current values for absent parameters, matching updt_params.
func (o *satTacticOptions) update(c *uparams.ParamsContext, p *uparams.ParamsRef) (err error) {
	if o == nil || c == nil || p == nil {
		return ast.ErrASTHandle
	}
	key := "extrapolate_strategy"
	if o.extrapolate, err = c.GetUIntName(p, &key, o.extrapolate); err != nil {
		return err
	}
	key = "projection_mode"
	if o.projection, err = c.GetBoolName(p, &key, o.projection); err != nil {
		return err
	}
	key = "strong_context_simplify"
	if o.strongSimplify, err = c.GetBoolName(p, &key, o.strongSimplify); err != nil {
		return err
	}
	key = "strong_context_simplify_local"
	o.simplifyLocal, err = c.GetBoolName(p, &key, o.simplifyLocal)
	return err
}

func collectSATTacticOptions(out *uparams.ParamDescrs) error {
	if out == nil {
		return ast.ErrASTHandle
	}
	descriptions := []struct {
		name string
		kind uparams.ParamKind
		text string
	}{
		{"extrapolate_strategy", uparams.ParamUInt, "(default: 0 trivial extrapolation) 1 - nnf strengthening 2 - smt-test 3 - nnf_weakening"},
		{"projection_mode", uparams.ParamBool, "(default: true - full) false - partial quantifier instantiation"},
		{"strong_context_simplify", uparams.ParamBool, "(default: true) use strong context simplifier on result of quantifier elimination"},
		{"strong_context_simplify_local", uparams.ParamBool, "(default: false) use strong context simplifier locally on the new formula only"},
	}
	for _, d := range descriptions {
		if err := out.InsertName(d.name, d.kind, &d.text, nil, nil); err != nil {
			return err
		}
	}
	return nil
}
