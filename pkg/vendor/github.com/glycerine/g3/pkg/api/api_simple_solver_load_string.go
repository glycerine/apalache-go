package api

// Source: src/api/api_solver.cpp:152-159 Z3_solver_from_string.

import "strings"

func (s *APISimpleSolver) LoadSMT2String(input string) error {
	return s.LoadSMT2(strings.NewReader(input))
}
