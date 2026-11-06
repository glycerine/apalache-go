package api

import "github.com/glycerine/g3/pkg/ast"

func (o *APIOptimize) OptimizationCompareModel(index uint32, md *APIModel, comparison OptimizationModelComparison) (ast.ASTRef, error) {
	if err := o.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if md == nil || md.owner != o.owner {
		return ast.ASTRef{}, ErrSMTContextOwner
	}
	if err := md.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return o.command.OptimizationCompareModel(index, md.core, comparison)
}

func (o *APIOptimize) OptimizationModelLE(index uint32, md *APIModel) (ast.ASTRef, error) {
	return o.OptimizationCompareModel(index, md, OptimizationModelLE)
}

func (o *APIOptimize) OptimizationModelGE(index uint32, md *APIModel) (ast.ASTRef, error) {
	return o.OptimizationCompareModel(index, md, OptimizationModelGE)
}

func (o *APIOptimize) OptimizationModelGT(index uint32, md *APIModel) (ast.ASTRef, error) {
	return o.OptimizationCompareModel(index, md, OptimizationModelGT)
}
