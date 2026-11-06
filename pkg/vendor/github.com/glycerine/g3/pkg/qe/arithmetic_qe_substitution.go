package qe

// Source: src/qe/qe_arith_plugin.cpp x_subst.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

type ArithmeticXSubstitution struct {
	self        *ArithmeticXSubstitution
	util        *ArithmeticQETermUtil
	term        ast.ASTRef
	coefficient *big.Rat
	closed      bool
}

func NewArithmeticXSubstitution(util *ArithmeticQETermUtil) (*ArithmeticXSubstitution, error) {
	if util == nil {
		return nil, ast.ErrASTHandle
	}
	if err := util.open(); err != nil {
		return nil, err
	}
	s := &ArithmeticXSubstitution{util: util, coefficient: big.NewRat(1, 1)}
	s.self = s
	return s, nil
}

func (s *ArithmeticXSubstitution) open() error {
	if s == nil || s.self != s || s.util == nil {
		return ast.ErrASTHandle
	}
	if s.closed {
		return arena.ErrClosed
	}
	return s.util.open()
}

func (s *ArithmeticXSubstitution) SetTerm(term ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	if s.term == (ast.ASTRef{}) {
		ref, err := s.util.manager.NewRef(term)
		if err != nil {
			return err
		}
		s.term = ref
		return nil
	}
	return s.util.manager.SetRef(s.term, term)
}

func (s *ArithmeticXSubstitution) SetCoefficient(coefficient *big.Rat) error {
	if err := s.open(); err != nil {
		return err
	}
	if coefficient == nil {
		return ast.ErrASTHandle
	}
	s.coefficient.Set(coefficient)
	return nil
}

func (s *ArithmeticXSubstitution) Term() (ast.ASTHandle, bool, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	if s.term == (ast.ASTRef{}) {
		return ast.ASTHandle{}, false, nil
	}
	h, err := s.util.manager.RefGet(s.term)
	return h, err == nil, err
}

func (s *ArithmeticXSubstitution) Coefficient() (*big.Rat, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return new(big.Rat).Set(s.coefficient), nil
}

// MakeTerm returns t + c*m_term, or t when no term has been installed.
func (s *ArithmeticXSubstitution) MakeTerm(c *big.Rat, term ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if resultErr = s.open(); resultErr != nil {
		return result, resultErr
	}
	if c == nil {
		return result, ast.ErrASTHandle
	}
	stored, found, err := s.Term()
	if err != nil || !found {
		if err != nil {
			return result, err
		}
		return s.util.manager.NewRef(term)
	}
	scaled, err := s.util.scale(c, stored)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, s.util.manager.DelRef(&scaled)) }()
	scaledHandle, err := s.util.manager.RefGet(scaled)
	if err != nil {
		return result, err
	}
	combined, err := s.util.manager.MkArithmeticAdd(term, scaledHandle)
	if err != nil {
		return result, err
	}
	return s.util.manager.NewRef(combined)
}

func (s *ArithmeticXSubstitution) MakeCoefficient(k *big.Rat) (*big.Rat, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if k == nil {
		return nil, ast.ErrASTHandle
	}
	return new(big.Rat).Mul(k, s.coefficient), nil
}

func (s *ArithmeticXSubstitution) Close() error {
	if s == nil || s.self != s {
		return ast.ErrASTHandle
	}
	if s.closed {
		return nil
	}
	if s.term != (ast.ASTRef{}) {
		if err := s.util.manager.DelRef(&s.term); err != nil {
			return err
		}
	}
	s.closed = true
	return nil
}
