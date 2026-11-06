package ast

// Source: src/ast/ast_smt2_pp.cpp:1063-1092,1139-1144.

import (
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// SMT2PPBooleanTreeWithParams is the Boolean-root specialization of the
// source pp_params-backed printer construction.
func (m *ASTManager) SMT2PPBooleanTreeWithParams(root ASTHandle, names []usymbol.Symbol, c *uparams.ParamsContext, g *uparams.GlobalParamsService, local *uparams.ParamsRef) (ASTHandle, error) {
	if m == nil || c == nil || g == nil || g.Context != c {
		return ASTHandle{}, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return ASTHandle{}, err
	}
	profile, err := SMT2PPExprProfileFromParams(c, g, local)
	if err != nil {
		return ASTHandle{}, err
	}
	return m.SMT2PPBooleanTreeWithConfig(root, names, SMT2PPExprConfig{Profile: &profile, Globals: g})
}
