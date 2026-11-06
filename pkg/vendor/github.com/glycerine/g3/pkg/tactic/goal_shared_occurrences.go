package tactic

// Source: src/tactic/goal_shared_occs.cpp; src/tactic/goal_shared_occs.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type GoalSharedOccurrences struct {
	manager *ast.ASTManager
	occs    *ast.ASTSharedOccurrences
}

func NewGoalSharedOccurrences(m *ast.ASTManager, trackAtomic, visitQuantifiers, visitPatterns bool) (*GoalSharedOccurrences, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	occs, err := ast.NewASTSharedOccurrencesWithConfig(m, ast.ASTSharedOccurrencesConfig{
		TrackAtomic:      trackAtomic,
		VisitQuantifiers: visitQuantifiers,
		VisitPatterns:    visitPatterns,
	})
	if err != nil {
		return nil, err
	}
	return &GoalSharedOccurrences{manager: m, occs: occs}, nil
}

func NewDefaultGoalSharedOccurrences(m *ast.ASTManager) (*GoalSharedOccurrences, error) {
	return NewGoalSharedOccurrences(m, false, true, false)
}

func (s *GoalSharedOccurrences) Collect(g *Goal) (result error) {
	if s == nil || s.occs == nil || g == nil || g.manager != s.manager {
		return ast.ErrASTHandle
	}
	if err := s.occs.Reset(); err != nil {
		return err
	}
	visited, err := ast.NewASTSharedOccurrencesMark(s.manager)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, visited.Close()) }()
	for _, form := range g.forms {
		if err = s.occs.CollectWithMark(form, visited); err != nil {
			return err
		}
	}
	return nil
}

func (s *GoalSharedOccurrences) IsShared(h ast.ASTHandle) (bool, error) {
	if s == nil || s.occs == nil {
		return false, ast.ErrASTHandle
	}
	return s.occs.IsShared(h)
}

func (s *GoalSharedOccurrences) NumShared() uint32 {
	if s == nil || s.occs == nil {
		return 0
	}
	return s.occs.NumShared()
}

func (s *GoalSharedOccurrences) Reset() error {
	if s == nil || s.occs == nil {
		return ast.ErrASTHandle
	}
	return s.occs.Reset()
}

func (s *GoalSharedOccurrences) Cleanup() error {
	if s == nil || s.occs == nil {
		return ast.ErrASTHandle
	}
	return s.occs.Cleanup()
}

func (s *GoalSharedOccurrences) Close() error {
	if s == nil || s.occs == nil {
		return ast.ErrASTHandle
	}
	err := s.occs.Close()
	if err == nil {
		s.occs = nil
		s.manager = nil
	}
	return err
}
