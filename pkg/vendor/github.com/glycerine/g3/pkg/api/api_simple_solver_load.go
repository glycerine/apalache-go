package api

// Source: src/api/api_solver.cpp:129-150 solver_from_stream.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/parsers/smt2"
)

// LoadSMT2 parses an SMT-LIB command stream and adds its final assertions to
// the solver. Check commands are syntax-checked but not executed. Parsing is
// completed before the solver is mutated.
func (s *APISimpleSolver) LoadSMT2(input io.Reader) (resultErr error) {
	if err := s.open(); err != nil {
		return err
	}
	if input == nil {
		return ErrSMTContextOwner
	}
	parser, err := (&SMTContext{s.owner}).newSMT2ParserContext(SMT2Bindings{})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, parser.Close()) }()
	if err = smt2.ParseAssertions(parser, input); err != nil {
		return err
	}
	assertions, err := parser.Assertions()
	if err != nil {
		return err
	}
	for _, assertion := range assertions {
		if err = s.solver.Assert(assertion); err != nil {
			return err
		}
	}
	return nil
}
