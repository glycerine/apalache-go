package sat

// Source: src/sat/dimacs.cpp concrete literal and clause stream consumed by
// api_solver.cpp's .dimacs file branch.

import "errors"

var ErrDIMACSParse = errors.New("sat: invalid DIMACS input")

type DIMACSLiteral int32

type DIMACSCNF struct {
	VariableCount uint32
	Clauses       [][]DIMACSLiteral
}
