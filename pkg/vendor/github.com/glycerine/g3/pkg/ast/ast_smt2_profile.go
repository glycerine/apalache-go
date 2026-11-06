package ast

// Source: src/ast/ast_smt2_pp.cpp:1063-1082.

import (
	"errors"

	uparams "github.com/glycerine/g3/pkg/util/params"
)

func defaultSMT2PPExprProfile() SMT2PPExprProfile {
	return SMT2PPExprProfile{DecimalPrecision: 10, BVLiterals: true, MaxDepth: 5, MinAliasSize: 10, FlatAssoc: true}
}

// SMT2PPExprProfileFromParams evaluates the source smt2_printer constructor's
// eight pp_params fields in declaration order and retires its module snapshot.
func SMT2PPExprProfileFromParams(c *uparams.ParamsContext, g *uparams.GlobalParamsService, local *uparams.ParamsRef) (profile SMT2PPExprProfile, result error) {
	p, err := NewPPParams(c, g, local)
	if err != nil {
		return profile, err
	}
	defer func() { result = errors.Join(result, p.Close()) }()
	if profile.Decimal, err = p.Decimal(); err != nil {
		return profile, err
	}
	if profile.DecimalPrecision, err = p.DecimalPrecision(); err != nil {
		return profile, err
	}
	if profile.BVLiterals, err = p.BVLiterals(); err != nil {
		return profile, err
	}
	if profile.FPRealLiterals, err = p.FPRealLiterals(); err != nil {
		return profile, err
	}
	if profile.BVNegative, err = p.BVNeg(); err != nil {
		return profile, err
	}
	if profile.MaxDepth, err = p.MaxDepth(); err != nil {
		return profile, err
	}
	if profile.MinAliasSize, err = p.MinAliasSize(); err != nil {
		return profile, err
	}
	profile.FlatAssoc, err = p.FlatAssoc()
	return profile, err
}
