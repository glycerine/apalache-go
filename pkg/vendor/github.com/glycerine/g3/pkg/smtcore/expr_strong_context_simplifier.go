package smtcore

// Source: src/smt/expr_context_simplifier.cpp/.h, strong simplifier lifecycle
// and scoped forced-value checks. The model-based traversal is in a separate file.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// ExprStrongContextSimplifier owns its solver and naming declaration. The
// manager and solver configuration are borrowed.
type ExprStrongContextSimplifier struct {
	self    *ExprStrongContextSimplifier
	manager *ast.ASTManager
	solver  *SMTSolver
	name    ast.ASTRef
	bytes   uint64
	closed  bool
}

func NewExprStrongContextSimplifier(m *ast.ASTManager, config SMTSolverConfig) (result *ExprStrongContextSimplifier, err error) {
	if m == nil || config.Params == nil {
		return nil, ast.ErrASTHandle
	}
	size := uint64(reflect.TypeOf(ExprStrongContextSimplifier{}).Size())
	if err = m.Budget().Reserve(size); err != nil {
		return nil, err
	}
	result = &ExprStrongContextSimplifier{manager: m, bytes: size}
	result.self = result
	defer func() {
		if err != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return result, err
	}
	integer, err := m.MkFamilySort(family, 1, nil)
	if err != nil {
		return result, err
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return result, err
	}
	decl, err := m.MkFuncDecl(symbol.Numerical(0xbeef101), []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		return result, err
	}
	result.name, err = m.NewRef(decl)
	if err != nil {
		return result, err
	}
	result.solver, err = NewSMTSolver(m, nil, "", config)
	return result, err
}

func (s *ExprStrongContextSimplifier) open() error {
	if s == nil || s.self != s {
		return ast.ErrASTHandle
	}
	if s.closed {
		return arena.ErrClosed
	}
	return nil
}

func (s *ExprStrongContextSimplifier) Push() error {
	if err := s.open(); err != nil {
		return err
	}
	return s.solver.Push()
}
func (s *ExprStrongContextSimplifier) Pop() error {
	if err := s.open(); err != nil {
		return err
	}
	return s.solver.Pop(1)
}
func (s *ExprStrongContextSimplifier) Assert(expression ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.solver.AssertFormula(expression)
}

// isForced checks unsatisfiability of expression = value under the current
// context, matching the source's polarity convention.
func (s *ExprStrongContextSimplifier) isForced(expression, value ast.ASTHandle) (forced bool, err error) {
	if err = s.open(); err != nil {
		return false, err
	}
	if err = s.solver.Push(); err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, s.solver.Pop(1)) }()
	equality, err := s.manager.MkEq(expression, value)
	if err != nil {
		return false, err
	}
	ref, err := s.manager.NewRef(equality)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, s.manager.DelRef(&ref)) }()
	if err = s.solver.AssertFormula(equality); err != nil {
		return false, err
	}
	status, err := s.solver.CheckSat(nil)
	return status == lbool.False, err
}

func (s *ExprStrongContextSimplifier) CollectStatistics(out *statistics.Statistics) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.solver.CollectStatistics(out)
}
func (s *ExprStrongContextSimplifier) ResetStatistics() error {
	if err := s.open(); err != nil {
		return err
	}
	return s.solver.ResetStatistics()
}
func (s *ExprStrongContextSimplifier) Close() error {
	if s == nil || s.self != s {
		return ast.ErrASTHandle
	}
	if s.closed {
		return nil
	}
	if s.solver != nil {
		if err := s.solver.Close(); err != nil {
			return err
		}
		s.solver = nil
	}
	if s.name != (ast.ASTRef{}) {
		if err := s.manager.DelRef(&s.name); err != nil {
			return err
		}
	}
	s.manager.Budget().Release(s.bytes)
	s.bytes = 0
	s.closed = true
	return nil
}
