package opt

// Source: src/opt/maxsmt.cpp expression-vector lifetime and objective handoff.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (s *weightedSoftSet) Release() (terms []ast.ASTRef, weights []*big.Rat, offset big.Rat, err error) {
	if s == nil || s.closed || s.manager == nil {
		return nil, nil, offset, ast.ErrASTHandle
	}
	terms, weights = s.terms, s.weights
	offset.Set(&s.offset)
	s.terms = nil
	s.weights = nil
	s.index = nil
	s.closed = true
	s.manager = nil
	return terms, weights, offset, nil
}

func (s *weightedSoftSet) Close() (result error) {
	if s == nil || s.closed {
		return nil
	}
	s.closed = true
	for i := len(s.terms) - 1; i >= 0; i-- {
		result = errors.Join(result, s.manager.DelRef(&s.terms[i]))
	}
	s.terms = nil
	s.weights = nil
	s.index = nil
	s.manager = nil
	return result
}
