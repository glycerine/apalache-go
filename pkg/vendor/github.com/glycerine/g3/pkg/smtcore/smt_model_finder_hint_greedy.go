package smtcore

// Source: src/smt/smt_model_finder.cpp:2833-2865,2910-2946.

import "github.com/glycerine/g3/pkg/ast"

const hintGreedyMaxDepth uint32 = 10

// searchFrom is hint_solver::process(func_decl*). A successful search returns
// with its selected definitions and satisfied set intact, as the source's
// found_satisfied_subset exception exits before branch rollback.
func (s *hintSearchState) searchFrom(m *ast.ASTManager, f ast.ASTHandle) (bool, error) {
	if s == nil || m == nil || s.residue == nil || s.satisfied == nil || !s.residue.empty() || !s.satisfied.empty() || len(s.definitions) != 0 {
		return false, ast.ErrASTDomain
	}
	return s.greedyFunction(m, f, 0)
}

func (s *hintSearchState) greedyFunction(m *ast.ASTManager, f ast.ASTHandle, depth uint32) (bool, error) {
	if depth >= hintGreedyMaxDepth {
		return false, nil
	}
	s.traceGreedyFunction(m, f, depth)
	defs := s.defsByFunction[f]
	if defs == nil {
		return false, ast.ErrASTDomain
	}
	for def := range defs {
		// Source asserts that a selected function is not chosen twice.
		if _, selected := s.definitions[f]; selected {
			return false, ast.ErrASTDomain
		}
		s.pushScope()
		if e := s.selectDefinition(f, def); e != nil {
			return false, e
		}
		useful, e := s.updateSatisfiedResidue(f, def)
		if e != nil {
			return false, e
		}
		if useful {
			found, e := s.greedyResidue(m, depth+1)
			if e != nil || found {
				return found, e
			}
		}
		if e := s.popScope(); e != nil {
			return false, e
		}
		s.forgetDefinition(f)
	}
	return false, nil
}

func (s *hintSearchState) greedyResidue(m *ast.ASTManager, depth uint32) (bool, error) {
	if s.residue.empty() {
		cyclic, e := s.isCyclic(m)
		if e == nil && !cyclic {
			s.traceFoundSubset(m)
		}
		return !cyclic && e == nil, e
	}
	candidates, e := s.candidatesFromResidue()
	if e != nil {
		return false, e
	}
	s.traceResidueCandidates(m, candidates)
	for f := range candidates {
		found, e := s.greedyFunction(m, f, depth)
		if e != nil || found {
			return found, e
		}
	}
	return false, nil
}
