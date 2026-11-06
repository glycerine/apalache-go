package api

// Source: src/api/api_opt.cpp model comparison consumers over
// src/opt/opt_context.cpp mk_le/mk_ge/mk_gt.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
)

type OptimizationModelComparison = cmd_context.OptimizationModelComparison

const (
	OptimizationModelLE = cmd_context.OptimizationModelLE
	OptimizationModelGE = cmd_context.OptimizationModelGE
	OptimizationModelGT = cmd_context.OptimizationModelGT
)

func (c *SMTContext) OptimizationCompareModel(index uint32, md *APIModel, comparison OptimizationModelComparison) (ast.ASTRef, error) {
	if err := c.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if md == nil || md.owner != c.smtContextState {
		return ast.ASTRef{}, ErrSMTContextOwner
	}
	if err := md.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return c.command.OptimizationCompareModel(index, md.core, comparison)
}

func (c *SMTContext) OptimizationModelLE(index uint32, md *APIModel) (ast.ASTRef, error) {
	return c.OptimizationCompareModel(index, md, OptimizationModelLE)
}

func (c *SMTContext) OptimizationModelGE(index uint32, md *APIModel) (ast.ASTRef, error) {
	return c.OptimizationCompareModel(index, md, OptimizationModelGE)
}

func (c *SMTContext) OptimizationModelGT(index uint32, md *APIModel) (ast.ASTRef, error) {
	return c.OptimizationCompareModel(index, md, OptimizationModelGT)
}
