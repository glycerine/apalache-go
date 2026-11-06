package api

// Source: src/api/api_solver.cpp:467-475.

import "strings"

func (s *APISimpleSolver) String() (string, error) {
	if err := s.open(); err != nil {
		return "", err
	}
	var output strings.Builder
	if err := s.solver.Display(&output, nil); err != nil {
		return "", err
	}
	return output.String(), nil
}
