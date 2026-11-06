package duality

// Source: src/duality/duality_solver.cpp, Duality constructor and SetOption.

import (
	"strconv"
	"strings"
)

// SolverOptions holds the concrete options consumed by the Duality solver.
type SolverOptions struct {
	FullExpand, NoConj, FeasibleEdges, UseUnderapprox       bool
	Report, StratifiedInlining, BatchExpand, EnableRestarts bool
	RecursionBound                                          int32
	ConjectureFile                                          string
}

// The source leaves EnableRestarts uninitialized; the fixedpoint interface
// explicitly sets it before solving, and Go initializes it to false.
func NewSolverOptions() *SolverOptions {
	return &SolverOptions{FeasibleEdges: true, UseUnderapprox: true, RecursionBound: -1}
}

// SetOption reports the source return value. The source stores conjecture_file
// but returns false for it; invalid Boolean values leave their option unchanged.
func (o *SolverOptions) SetOption(name, value string) bool {
	if o == nil {
		return false
	}
	if name == "conjecture_file" {
		o.ConjectureFile = value
		return false
	}
	if name == "recursion_bound" {
		o.RecursionBound = parseRecursionBound(value)
		return true
	}
	var target *bool
	switch name {
	case "full_expand":
		target = &o.FullExpand
	case "no_conj":
		target = &o.NoConj
	case "feasible_edges":
		target = &o.FeasibleEdges
	case "use_underapprox":
		target = &o.UseUnderapprox
	case "report":
		target = &o.Report
	case "stratified_inlining":
		target = &o.StratifiedInlining
	case "batch_expand":
		target = &o.BatchExpand
	case "enable_restarts":
		target = &o.EnableRestarts
	default:
		return false
	}
	switch value {
	case "0":
		*target = false
		return true
	case "1":
		*target = true
		return true
	default:
		return false
	}
}

// parseRecursionBound uses Go's integer parser for the decimal prefix accepted
// by the source caller, including leading whitespace and an optional sign.
func parseRecursionBound(value string) int32 {
	value = strings.TrimLeft(value, " \t\n\r\v\f")
	end := 0
	if len(value) > 0 && (value[0] == '+' || value[0] == '-') {
		end++
	}
	start := end
	for end < len(value) && value[end] >= '0' && value[end] <= '9' {
		end++
	}
	if end == start {
		return 0
	}
	parsed, _ := strconv.ParseInt(value[:end], 10, 32)
	return int32(parsed)
}
