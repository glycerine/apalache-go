package api

// Source: the SMT-LIB branch of src/api/api_solver.cpp:161-185.
// Native .dimacs loading remains gated on the retained DIMACS parser.

import (
	"errors"
	"os"
)

func (s *APISimpleSolver) LoadSMT2File(path string) (resultErr error) {
	input, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, input.Close()) }()
	return s.LoadSMT2(input)
}
