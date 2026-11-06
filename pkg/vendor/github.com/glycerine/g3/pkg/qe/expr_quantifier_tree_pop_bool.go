package qe

// Source: src/qe/qe.cpp, pop for the Boolean solver-plugin subset.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
)

type exprQEPopStatus uint8

const (
	exprQEPopCompleted exprQEPopStatus = iota
	exprQEPopNeedsPartition
	exprQEPopBranchSelected
	exprQEPopNoBranch
)

// popExprQEBoolTree solves Boolean units, blocks completed paths, and routes
// remaining nodes through partition analysis and variable selection. It does
// not yet perform the recursive independent-partition elimination.
func popExprQEBoolTree(node *exprQETree, variables *exprQEBranchVariables, context *SolverContext, plugin *BoolPlugin, partitioner *ConjunctionPartitioner, solver *smtcore.SMTSolver, evaluator *model.ModelEvaluator, modelCore *model.ModelCore, config Config, rw *rewriter.TheoryRewriter, nnf *QENNFNormalizer, freeVariables *[]ast.ASTRef, withDefinitions bool) (result *exprQETree, status exprQEPopStatus, err error) {
	current, err := solveExprQEBoolUnits(node, variables, plugin, config, rw, nnf)
	if err != nil {
		return nil, exprQEPopNoBranch, err
	}
	blocked, err := blockExprQETreeIfSolved(current, solver)
	if err != nil {
		return nil, exprQEPopNoBranch, err
	}
	if blocked {
		return current, exprQEPopCompleted, nil
	}
	part, err := partitionExprQETree(current, variables, partitioner)
	if err != nil {
		return nil, exprQEPopNoBranch, err
	}
	defer func() { err = errors.Join(err, part.Close()) }()
	if part.split {
		return current, exprQEPopNeedsPartition, nil
	}
	child, handled, err := closeExprQETreePartition(current, part, solver, evaluator, modelCore, config, withDefinitions)
	if err != nil {
		return nil, exprQEPopNoBranch, err
	}
	if handled {
		return child, exprQEPopCompleted, nil
	}
	selected, err := chooseExprQETreeBranch(current, variables, context, solver, freeVariables)
	if err != nil {
		return nil, exprQEPopNoBranch, err
	}
	if selected {
		return current, exprQEPopBranchSelected, nil
	}
	return current, exprQEPopNoBranch, nil
}
