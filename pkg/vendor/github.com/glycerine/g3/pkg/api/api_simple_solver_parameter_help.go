package api

// Source: src/api/api_solver.cpp:188-203.

import "errors"

func (s *APISimpleSolver) ParameterHelp() (text string, resultErr error) {
	descriptions, err := s.ParameterDescriptions()
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, descriptions.Close()) }()
	return descriptions.value.Display(0, false, true)
}
