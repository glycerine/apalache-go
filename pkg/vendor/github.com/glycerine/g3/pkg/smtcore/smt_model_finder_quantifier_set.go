package smtcore

// Source: src/util/backtrackable_set.h:28-100; concrete qset/qsset uses in
// src/smt/smt_model_finder.cpp:2739-2864.

import "github.com/glycerine/g3/pkg/ast"

type quantifierSetTrail struct {
	value    ast.ASTHandle
	inserted bool
}

// scopedQuantifierSet is the concrete quantifier-pointer set used by the
// model-finder hint solver. Handles are borrowed; the AST owner retains them.
// onUndo is nil for m_residue and resets quantifier_info's chosen function
// for m_satisfied. The source invokes it before reversing each scoped edit.
type scopedQuantifierSet struct {
	values map[ast.ASTHandle]struct{}
	trail  []quantifierSetTrail
	scopes []int
	onUndo func(ast.ASTHandle, bool)
}

func newScopedQuantifierSet(onUndo func(ast.ASTHandle, bool)) *scopedQuantifierSet {
	return &scopedQuantifierSet{
		values: make(map[ast.ASTHandle]struct{}),
		onUndo: onUndo,
	}
}

func (s *scopedQuantifierSet) contains(q ast.ASTHandle) bool {
	_, found := s.values[q]
	return found
}

func (s *scopedQuantifierSet) empty() bool { return len(s.values) == 0 }

func (s *scopedQuantifierSet) insert(q ast.ASTHandle) {
	if s.contains(q) {
		return
	}
	s.values[q] = struct{}{}
	if len(s.scopes) != 0 {
		s.trail = append(s.trail, quantifierSetTrail{value: q, inserted: true})
	}
}

func (s *scopedQuantifierSet) erase(q ast.ASTHandle) {
	if len(s.scopes) == 0 {
		// Source line 62 calls insert in the level-zero erase branch.
		s.insert(q)
		return
	}
	if !s.contains(q) {
		return
	}
	delete(s.values, q)
	s.trail = append(s.trail, quantifierSetTrail{value: q})
}

func (s *scopedQuantifierSet) pushScope() {
	s.scopes = append(s.scopes, len(s.trail))
}

func (s *scopedQuantifierSet) popScope() error {
	if len(s.scopes) == 0 {
		return ast.ErrASTDomain
	}
	last := len(s.scopes) - 1
	stop := s.scopes[last]
	s.scopes = s.scopes[:last]
	for len(s.trail) > stop {
		i := len(s.trail) - 1
		edit := s.trail[i]
		if s.onUndo != nil {
			s.onUndo(edit.value, edit.inserted)
		}
		if edit.inserted {
			delete(s.values, edit.value)
		} else {
			s.values[edit.value] = struct{}{}
		}
		s.trail = s.trail[:i]
	}
	return nil
}

func (s *scopedQuantifierSet) reset() {
	clear(s.values)
	s.trail = s.trail[:0]
	s.scopes = s.scopes[:0]
}

func (s *scopedQuantifierSet) snapshot() []ast.ASTHandle {
	result := make([]ast.ASTHandle, 0, len(s.values))
	for q := range s.values {
		result = append(result, q)
	}
	return result
}
