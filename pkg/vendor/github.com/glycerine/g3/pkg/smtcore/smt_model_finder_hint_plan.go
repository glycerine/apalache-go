package smtcore

// Source: src/smt/smt_model_finder.cpp:2609-2616,2948-2996.

import "github.com/glycerine/g3/pkg/ast"

// hintSubsetPlan is the search result before the enclosing model finder
// publishes each selected function definition as an interpretation.
type hintSubsetPlan struct {
	remaining   []ast.ASTHandle
	definitions map[ast.ASTHandle]ast.ASTHandle
}

// resetRun releases the per-search indexes and scoped sets. The quantifier
// information is borrowed from the enclosing model finder and survives runs.
func (s *hintSearchState) resetRun() error {
	if s == nil || s.residue == nil || s.satisfied == nil {
		return ast.ErrASTDomain
	}
	clear(s.qByFunction)
	clear(s.qByDefinition)
	clear(s.defsByFunction)
	clear(s.forbidden)
	clear(s.candidates)
	clear(s.definitions)
	s.satisfied.reset()
	s.residue.reset()
	return nil
}

// planSubset follows hint_solver::process through preprocessing, indexed
// search, and the new_qs partition. The caller must publish definitions before
// treating a found plan as a completed model-finder hint pass.
func (s *hintSearchState) planSubset(m *ast.ASTManager, qs []ast.ASTHandle) (hintSubsetPlan, bool, error) {
	var plan hintSubsetPlan
	if m == nil {
		return plan, false, ast.ErrASTDomain
	}
	if e := s.resetRun(); e != nil {
		return plan, false, e
	}
	candidates, rejected, e := s.preprocess(qs)
	if e != nil {
		return plan, false, e
	}
	plan.remaining = rejected
	if len(candidates) == 0 {
		return plan, false, nil
	}
	if e := s.makeQuantifierIndexes(candidates); e != nil {
		return plan, false, e
	}
	for f := range s.candidates {
		found, e := s.searchFrom(m, f)
		if e != nil {
			return plan, false, e
		}
		if found {
			for _, q := range candidates {
				if !s.satisfied.contains(q) {
					plan.remaining = append(plan.remaining, q)
				}
			}
			plan.definitions = make(map[ast.ASTHandle]ast.ASTHandle, len(s.definitions))
			for selected, def := range s.definitions {
				plan.definitions[selected] = def
			}
			return plan, true, nil
		}
	}
	plan.remaining = append(plan.remaining, candidates...)
	return plan, false, nil
}
