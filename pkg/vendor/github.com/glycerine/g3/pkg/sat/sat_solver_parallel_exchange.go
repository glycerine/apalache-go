package sat

import "github.com/glycerine/g3/pkg/util/lbool"

func (s *satSolverConflictResolution) exchangeParallelBound() error {
	if s.parallel == nil {
		return ErrSATMissingParallelBinding
	}
	rootSize, err := s.solver.sourceInitTrailSize()
	if err != nil {
		return err
	}
	var out []Literal
	for i := s.parallelLimitOut; i < rootSize; i++ {
		literal, readErr := s.analysis.propagation.variables.trailRead(i)
		if readErr != nil {
			return readErr
		}
		if uint32(literal.variable()) < s.parallelNumVars {
			out = append(out, literal)
		}
	}
	s.parallelLimitOut = rootSize
	in, limit, err := s.parallel.Exchange(out, s.parallelLimitIn)
	if err != nil {
		return err
	}
	s.parallelLimitIn = limit
	p := s.analysis.propagation
	for _, literal := range in {
		if p.inconsistent {
			break
		}
		if uint32(literal.variable()) >= s.parallelNumVars {
			return ErrSATAssertion
		}
		level, readErr := p.variables.levelVar(literal.variable())
		if readErr != nil {
			return readErr
		}
		value, readErr := p.variables.valueLiteral(literal)
		if readErr != nil {
			return readErr
		}
		if level != 0 || value != lbool.True {
			if err = p.assign(literal, satJustification{}); err != nil {
				return err
			}
		}
	}
	return nil
}
