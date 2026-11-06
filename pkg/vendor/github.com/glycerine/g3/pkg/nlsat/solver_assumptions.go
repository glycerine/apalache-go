package nlsat

// Source: src/nlsat/nlsat_solver.cpp:1354-1416.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *Solver) clauseUsesInputLiteral(c *Clause, inputs map[*sat.Literal]struct{}) (bool, error) {
	if c == nil {
		return false, arena.ErrConfig
	}
	var dependencies []Assumption
	if err := s.assumptions.linearize(c.Assumptions(), &dependencies); err != nil {
		return false, err
	}
	for _, dependency := range dependencies {
		if _, found := inputs[dependency.InputLiteral()]; found {
			return true, nil
		}
	}
	return false, nil
}

func (s *Solver) collectInputLiteralClauses(inputs map[*sat.Literal]struct{}) error {
	for _, clauses := range [][]*Clause{
		append([]*Clause(nil), s.clauses.originals...),
		append([]*Clause(nil), s.clauses.learned...),
	} {
		for _, clause := range clauses {
			found, err := s.clauseUsesInputLiteral(clause, inputs)
			if err != nil {
				return err
			}
			if found {
				if err = s.clauses.deleteClause(clause); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *Solver) cleanupInputLiteralClauses(inputs map[*sat.Literal]struct{}) error {
	if err := s.clauses.cleanup(); err != nil {
		return err
	}
	if err := s.collectInputLiteralClauses(inputs); err != nil {
		return errors.Join(err, s.clauses.cleanup())
	}
	return s.clauses.cleanup()
}

// CheckAssumptions checks temporary unit literals and replaces assumptions
// with the subset used by an UNSAT result. SAT and unknown results clear it.
// Each input element has distinct source identity even when values repeat.
func (s *Solver) CheckAssumptions(assumptions *[]sat.Literal) (result lbool.LBool, resultErr error) {
	if err := s.open(); err != nil {
		return lbool.Undef, err
	}
	if assumptions == nil {
		return lbool.Undef, arena.ErrConfig
	}
	input := *assumptions
	identities := make(map[*sat.Literal]struct{}, len(input))
	for i := range input {
		identities[&input[i]] = struct{}{}
		if err := s.clauses.mkInputClause([]sat.Literal{input[i]}, literalAssumption(&input[i])); err != nil {
			resultErr = err
			break
		}
	}
	defer func() {
		resultErr = errors.Join(resultErr, s.cleanupInputLiteralClauses(identities))
	}()
	if resultErr != nil {
		return lbool.Undef, resultErr
	}
	result, resultErr = s.Check()
	if resultErr != nil {
		return result, resultErr
	}
	if result != lbool.False {
		*assumptions = (*assumptions)[:0]
		return result, resultErr
	}
	var dependencies []Assumption
	if resultErr = s.assumptions.linearize(s.resolution.lemmaAssumptions, &dependencies); resultErr != nil {
		return lbool.Undef, resultErr
	}
	core := make([]sat.Literal, 0, len(dependencies))
	for _, dependency := range dependencies {
		literal := dependency.InputLiteral()
		if _, found := identities[literal]; found {
			core = append(core, *literal)
		}
	}
	*assumptions = append((*assumptions)[:0], core...)
	return result, nil
}
