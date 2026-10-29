package smtcore

// Source: src/smt/smt_model_finder.cpp:1624-1665,1696-1707,
// 2585-2645,2648-2704,2817-2831.

import "github.com/glycerine/g3/pkg/ast"

// These are the hint solver's reached quantifier-info fields. The enclosing
// model finder owns the AST nodes and full cond_macro/quantifier_info objects;
// this search state borrows their identities and selected flags.
type hintConditionalMacro struct {
	function, definition ast.ASTHandle
	satisfyAtom          bool
}

type hintQuantifierInfo struct {
	nonGround []ast.ASTHandle
	macros    []hintConditionalMacro
	theOne    ast.ASTHandle
}

type hintFunctionDefinition struct {
	function, definition ast.ASTHandle
}

func (s *hintSearchState) info(q ast.ASTHandle) (*hintQuantifierInfo, error) {
	if s == nil || s.infos == nil || s.infos[q] == nil {
		return nil, ast.ErrASTDomain
	}
	return s.infos[q], nil
}

func (s *hintSearchState) isCandidate(q ast.ASTHandle) (bool, error) {
	info, e := s.info(q)
	if e != nil {
		return false, e
	}
	for _, macro := range info.macros {
		if macro.satisfyAtom {
			if _, forbidden := s.forbidden[macro.function]; !forbidden {
				return true, nil
			}
		}
	}
	return false, nil
}

func (s *hintSearchState) registerDeclsAsForbidden(q ast.ASTHandle) error {
	info, e := s.info(q)
	if e != nil {
		return e
	}
	for _, f := range info.nonGround {
		s.forbidden[f] = struct{}{}
	}
	return nil
}

// preprocess repeats the source filter until every remaining quantifier has
// an allowed satisfying macro. Rejected quantifiers forbid their non-ground
// declarations immediately, so later entries in the same pass see them.
func (s *hintSearchState) preprocess(qs []ast.ASTHandle) (candidates, rejected []ast.ASTHandle, err error) {
	if s == nil || s.forbidden == nil {
		return nil, nil, ast.ErrASTDomain
	}
	current := append([]ast.ASTHandle(nil), qs...)
	for {
		for _, q := range current {
			ok, e := s.isCandidate(q)
			if e != nil {
				return candidates, rejected, e
			}
			if ok {
				candidates = append(candidates, q)
			} else {
				if e := s.registerDeclsAsForbidden(q); e != nil {
					return candidates, rejected, e
				}
				rejected = append(rejected, q)
			}
		}
		if len(current) == len(candidates) {
			return candidates, rejected, nil
		}
		current, candidates = candidates, nil
	}
}

func (s *hintSearchState) insertQF(q, f ast.ASTHandle) {
	set := s.qByFunction[f]
	if set == nil {
		set = make(map[ast.ASTHandle]struct{})
		s.qByFunction[f] = set
	}
	set[q] = struct{}{}
}

func (s *hintSearchState) insertQFDef(q, f, def ast.ASTHandle) {
	key := hintFunctionDefinition{f, def}
	set := s.qByDefinition[key]
	if set == nil {
		set = make(map[ast.ASTHandle]struct{})
		s.qByDefinition[key] = set
		defs := s.defsByFunction[f]
		if defs == nil {
			defs = make(map[ast.ASTHandle]struct{})
			s.defsByFunction[f] = defs
		}
		defs[def] = struct{}{}
	}
	set[q] = struct{}{}
}

// makeQuantifierIndexes is hint_solver::mk_q_f_defs after preprocessing.
func (s *hintSearchState) makeQuantifierIndexes(qs []ast.ASTHandle) error {
	if s == nil || s.qByFunction == nil {
		return ast.ErrASTDomain
	}
	for _, q := range qs {
		info, e := s.info(q)
		if e != nil {
			return e
		}
		for _, f := range info.nonGround {
			if _, forbidden := s.forbidden[f]; !forbidden {
				s.insertQF(q, f)
			}
		}
		for _, macro := range info.macros {
			if !macro.satisfyAtom {
				continue
			}
			if _, forbidden := s.forbidden[macro.function]; forbidden {
				continue
			}
			s.insertQFDef(q, macro.function, macro.definition)
			s.candidates[macro.function] = struct{}{}
		}
	}
	return nil
}

// candidatesFromResidue follows get_candidates_from_residue. The source set
// has no ordering contract; a Go map likewise represents unique candidates.
func (s *hintSearchState) candidatesFromResidue() (map[ast.ASTHandle]struct{}, error) {
	if s == nil || s.residue == nil {
		return nil, ast.ErrASTDomain
	}
	result := make(map[ast.ASTHandle]struct{})
	for q := range s.residue.values {
		info, e := s.info(q)
		if e != nil {
			return nil, e
		}
		for _, macro := range info.macros {
			f := macro.function
			if !macro.satisfyAtom {
				continue
			}
			if _, forbidden := s.forbidden[f]; forbidden {
				continue
			}
			if _, selected := s.definitions[f]; selected {
				continue
			}
			result[f] = struct{}{}
		}
	}
	return result, nil
}
