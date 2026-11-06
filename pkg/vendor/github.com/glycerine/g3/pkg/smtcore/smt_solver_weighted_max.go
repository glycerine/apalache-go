package smtcore

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

// EnsureWeightedMaxTheory installs the MaxSMT weighted slack theory in the
// current solver, including scopes established by optimizer setup.
func (s *SMTSolver) EnsureWeightedMaxTheory(filter *tactic.FilterModelConverter) (*SMTWeightedMaxTheory, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	family, err := s.manager.EnsureFamilyID("weighted_maxsat")
	if err != nil {
		return nil, err
	}
	state := s.kernel.internalizer
	if existing := state.theories[family]; existing != nil {
		theory, ok := existing.(*SMTWeightedMaxTheory)
		if !ok {
			return nil, ast.ErrASTDomain
		}
		return theory, theory.ResetLocal()
	}
	theory, err := NewSMTWeightedMaxTheory(family, filter)
	if err != nil {
		return nil, err
	}
	if err = theory.Initialize(state); err != nil {
		return nil, err
	}
	state.theories[family] = theory
	state.theoryOrder = append(state.theoryOrder, theory)
	var pushed uint32
	for range state.scopes {
		if err = theory.PushScope(); err != nil {
			if pushed != 0 {
				err = errors.Join(err, theory.PopScopes(pushed))
			}
			state.theoryOrder = state.theoryOrder[:len(state.theoryOrder)-1]
			delete(state.theories, family)
			return nil, err
		}
		pushed++
	}
	return theory, nil
}
