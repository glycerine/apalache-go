package opt

// Source: src/opt/opt_context.cpp is_maxsat negative-weight normalization and
// src/opt/maxsmt.cpp maxsmt::add duplicate accumulation.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (s *weightedSoftSet) Add(term ast.ASTHandle, weight *big.Rat) error {
	if s == nil || s.closed || s.manager == nil || weight == nil {
		return ast.ErrASTHandle
	}
	if weight.Sign() == 0 {
		return nil
	}
	normalized := term
	positive := new(big.Rat).Set(weight)
	if positive.Sign() < 0 {
		s.offset.Add(&s.offset, positive)
		positive.Neg(positive)
		var err error
		normalized, err = s.manager.MkNot(term)
		if err != nil {
			return err
		}
	}
	if index, found := s.index[normalized]; found {
		s.weights[index].Add(s.weights[index], positive)
		return nil
	}
	ref, err := s.manager.NewRef(normalized)
	if err != nil {
		return err
	}
	s.index[normalized] = len(s.terms)
	s.terms = append(s.terms, ref)
	s.weights = append(s.weights, positive)
	return nil
}
