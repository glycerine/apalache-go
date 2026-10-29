package sat

// Source: src/sat/sat_solver.h:477-503; retained inc_sat_solver consumers.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// GetConsequences exposes the actual SAT consequence algorithm to its concrete
// incremental solver consumer. Returned rows own independent Go slice storage;
// even an error returns the algorithm's completed output prefix.
func (s *Solver) GetConsequences(asms []Literal, variables []BoolVar) (status lbool.LBool, rows [][]Literal, err error) {
	if err = s.open(); err != nil {
		return lbool.Undef, nil, err
	}
	out, err := newSATSolverLiteralRows(s.runtime.memory, s.budget)
	if err != nil {
		return lbool.Undef, nil, err
	}
	defer func() { rows = out.rows; err = errors.Join(err, out.CloseChecked()) }()
	status, err = s.getConsequences(asms, variables, out)
	return status, nil, err
}

// DisplayWCNF exposes the existing native SAT weighted-clause printer to the
// retained incremental solver. The stream is borrowed and preserves prefixes.
func (s *Solver) DisplayWCNF(out *utext.NumericTextStream, literals []Literal, weights []uint32) error {
	return s.displayWCNF(out, uint32(len(literals)), literals, weights)
}

func (s *Solver) DisplayAssignment(out *utext.NumericTextStream) error {
	return s.displayAssignment(out)
}
func (s *Solver) DisplayInto(out *utext.NumericTextStream) error { return s.displayInto(out) }

// FindMutexes runs the retained binary-clause clique algorithm on the existing
// SAT graph. It does not assert or internalize the requested literals.
func (s *Solver) FindMutexes(literals []Literal) (status lbool.LBool, rows [][]Literal, err error) {
	if err = s.open(); err != nil {
		return lbool.Undef, nil, err
	}
	out, err := newSATSolverLiteralRows(s.runtime.memory, s.budget)
	if err != nil {
		return lbool.Undef, nil, err
	}
	defer func() { rows = out.rows; err = errors.Join(err, out.CloseChecked()) }()
	status, err = s.findMutexes(literals, out)
	return status, nil, err
}
