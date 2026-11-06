package api

// Source: src/api/api_solver.cpp:161-185 Z3_solver_from_file.

import (
	"errors"
	"os"
	"path/filepath"
)

func (s *APISimpleSolver) LoadFile(path string) (resultErr error) {
	input, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, input.Close()) }()
	if filepath.Ext(path) == ".dimacs" {
		return s.LoadDIMACS(input)
	}
	return s.LoadSMT2(input)
}
