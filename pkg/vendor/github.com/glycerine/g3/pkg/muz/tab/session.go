package tab

import "github.com/glycerine/g3/pkg/ast"

// Session owns source imp state that intentionally survives between queries:
// predicate-selection cache/decay and monotonically increasing clause IDs.
type Session struct {
	manager      *ast.ASTManager
	selection    *predicateSelection
	nextSequence uint32
	closed       bool
}

func NewSession(manager *ast.ASTManager, strategy string) (*Session, error) {
	selection, err := newPredicateSelection(manager, strategy)
	if err != nil {
		return nil, err
	}
	return &Session{manager: manager, selection: selection}, nil
}

func (s *Session) open(manager *ast.ASTManager) error {
	if s == nil || s.closed || s.manager == nil || s.manager != manager || s.selection == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (s *Session) Close() error {
	if s == nil || s.closed {
		return nil
	}
	if err := s.selection.Close(); err != nil {
		return err
	}
	s.selection = nil
	s.manager = nil
	s.closed = true
	return nil
}
