package smtcore

// Source: src/smt/expr_context_simplifier.cpp, model-based assignment map.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type strongContextAssignments struct {
	manager *ast.ASTManager
	refs    []ast.ASTRef
	values  map[ast.ASTHandle]lbool.LBool
}

func (s *ExprStrongContextSimplifier) captureAssignments(formula ast.ASTHandle) (snapshot *strongContextAssignments, status lbool.LBool, err error) {
	if err = s.open(); err != nil {
		return nil, status, err
	}
	snapshot = &strongContextAssignments{manager: s.manager, values: make(map[ast.ASTHandle]lbool.LBool)}
	defer func() {
		if err != nil {
			err = errors.Join(err, snapshot.Close())
			snapshot = nil
		}
	}()
	if err = s.solver.Push(); err != nil {
		return snapshot, status, err
	}
	defer func() { err = errors.Join(err, s.solver.Pop(1)) }()
	if err = s.solver.AssertFormula(formula); err != nil {
		return snapshot, status, err
	}
	status, err = s.solver.CheckSat(nil)
	if err != nil || status == lbool.False {
		return snapshot, status, err
	}
	assignments, err := s.solver.Assignments()
	if err != nil {
		return snapshot, status, err
	}
	for _, literal := range assignments {
		ref, e := s.manager.NewRef(literal)
		if e != nil {
			return snapshot, status, e
		}
		snapshot.refs = append(snapshot.refs, ref)
		negated, arg, e := s.manager.IsNot(literal)
		if e != nil {
			return snapshot, status, e
		}
		if negated {
			snapshot.values[arg] = lbool.False
		} else {
			snapshot.values[literal] = lbool.True
		}
	}
	if err = snapshot.propagate(formula); err != nil {
		return snapshot, status, err
	}
	return snapshot, status, nil
}

// propagate follows the source postorder traversal and its three-valued
// Boolean rules. The formula remains borrowed while the snapshot is used.
func (s *strongContextAssignments) propagate(formula ast.ASTHandle) error {
	if s == nil || s.manager == nil || s.values == nil {
		return ast.ErrASTHandle
	}
	m := s.manager
	todo := []ast.ASTHandle{formula}
	for len(todo) != 0 {
		node := todo[len(todo)-1]
		kind, err := m.GetKind(node)
		if err != nil {
			return err
		}
		if kind != ast.ASTApp {
			s.values[node] = lbool.Undef
			todo = todo[:len(todo)-1]
			continue
		}
		count, err := m.GetNumArgs(node)
		if err != nil {
			return err
		}
		args := make([]ast.ASTHandle, count)
		visited := true
		for i := range args {
			args[i], err = m.GetArg(node, uint32(i))
			if err != nil {
				return err
			}
			if _, ok := s.values[args[i]]; !ok {
				todo = append(todo, args[i])
				visited = false
			}
		}
		if !visited {
			continue
		}
		todo = todo[:len(todo)-1]
		family, err := m.GetFamilyID(node)
		if err != nil {
			return err
		}
		op, err := m.GetDeclKind(node)
		if err != nil {
			return err
		}
		value := lbool.Undef
		if family == m.BasicFamilyID() {
			switch op {
			case ast.ASTOpAnd:
				value = lbool.True
				for _, arg := range args {
					if s.values[arg] == lbool.False {
						value = lbool.False
						break
					}
					if s.values[arg] == lbool.Undef {
						value = lbool.Undef
					}
				}
			case ast.ASTOpOr:
				value = lbool.False
				for _, arg := range args {
					if s.values[arg] == lbool.True {
						value = lbool.True
						break
					}
					if s.values[arg] == lbool.Undef {
						value = lbool.Undef
					}
				}
			case ast.ASTOpNot:
				value = s.values[args[0]].Negated()
			case ast.ASTOpImplies:
				left, right := s.values[args[0]], s.values[args[1]]
				if left == lbool.False || right == lbool.True {
					value = lbool.True
				} else if left == lbool.True && right == lbool.False {
					value = lbool.False
				}
			case ast.ASTOpIff, ast.ASTOpEq:
				left, right := s.values[args[0]], s.values[args[1]]
				if left != lbool.Undef && right != lbool.Undef {
					value = lbool.FromBool(left == right)
				}
			}
		}
		s.values[node] = value
	}
	return nil
}

func (s *strongContextAssignments) Close() (err error) {
	if s == nil || s.manager == nil {
		return nil
	}
	for len(s.refs) != 0 {
		i := len(s.refs) - 1
		if err = s.manager.DelRef(&s.refs[i]); err != nil {
			return err
		}
		s.refs = s.refs[:i]
	}
	s.manager = nil
	s.values = nil
	return nil
}
