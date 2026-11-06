package smtcore

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
)

type SMTImpliedEqualityStats struct {
	PairChecks, ValueEvaluations, ValueBuckets, FallbackTerms uint32
}

type smtImpliedEqualityEvaluator interface {
	EvaluateValue(ast.ASTHandle) (ast.ASTRef, error)
	Close() error
}

type smtImpliedEqualityEvaluatorSource interface {
	newImpliedEqualityEvaluator() (smtImpliedEqualityEvaluator, error)
}

type smtImpliedEqualityEngine struct {
	solver  solver.Solver
	manager *ast.ASTManager
	terms   []ast.ASTHandle
	bySort  map[ast.ASTHandle][]uint32
	classes smtImpliedEqualityClasses
	stats   SMTImpliedEqualityStats
}

func newSMTImpliedEqualityEngine(s solver.Solver, m *ast.ASTManager, terms []ast.ASTHandle) *smtImpliedEqualityEngine {
	return &smtImpliedEqualityEngine{solver: s, manager: m, terms: terms, bySort: make(map[ast.ASTHandle][]uint32), classes: newSMTImpliedEqualityClasses(len(terms))}
}
