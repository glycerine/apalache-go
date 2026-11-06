package clp

// Source: src/muz/clp/clp_context.h:25-41; clp_context.cpp:25-52.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var ErrCLP = errors.New("muz/clp: invalid bounded search")

// ErrSolverUnknown matches the source CLP solver-check failure. A search depth
// limit instead returns lbool.Undef without an error.
var ErrSolverUnknown = errors.New("undef")

type Solver interface {
	Push() error
	Pop(uint32) error
	ScopeLevel() (uint32, error)
	Assert(ast.ASTHandle) error
	CheckSat([]ast.ASTHandle) (lbool.LBool, error)
}

type Statistics struct{ Unfold, NoUnfold, Subsumed uint64 }

type Engine struct {
	manager *ast.ASTManager
	rules   *base.RuleSet
	solver  Solver
	depth   uint32
	stats   Statistics
	closed  bool
}

func NewEngine(m *ast.ASTManager, rules *base.RuleSet, solver Solver, depth uint32) (*Engine, error) {
	if m == nil || rules == nil || solver == nil {
		return nil, ErrCLP
	}
	if depth == 0 {
		depth = 20
	}
	return &Engine{manager: m, rules: rules, solver: solver, depth: depth}, nil
}
func (e *Engine) open() error {
	if e == nil || e.closed || e.manager == nil || e.rules == nil || e.solver == nil {
		return ErrCLP
	}
	return nil
}
