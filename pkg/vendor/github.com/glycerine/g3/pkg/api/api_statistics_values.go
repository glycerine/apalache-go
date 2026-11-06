package api

// Source: src/api/api_stats.cpp:101-130.

import "github.com/glycerine/g3/pkg/ast"

func (s *APIStatistics) UintValue(index uint32) (uint32, error) {
	i, err := s.index(index)
	if err != nil {
		return 0, err
	}
	if !s.value.IsUint(i) {
		return 0, ast.ErrASTDomain
	}
	return s.value.UintValue(i), nil
}

func (s *APIStatistics) DoubleValue(index uint32) (float64, error) {
	i, err := s.index(index)
	if err != nil {
		return 0, err
	}
	if s.value.IsUint(i) {
		return 0, ast.ErrASTDomain
	}
	return s.value.DoubleValue(i), nil
}
