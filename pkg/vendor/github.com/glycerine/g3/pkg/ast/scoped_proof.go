package ast

// Source: src/ast/scoped_proof.h.

// ScopedProofMode temporarily selects a manager proof-generation mode and
// restores the previous mode when closed. It owns no AST references.
type ScopedProofMode struct {
	self     *ScopedProofMode
	manager  *ASTManager
	previous ASTProofMode
	closed   bool
}

func newScopedProofMode(m *ASTManager, mode ASTProofMode) (*ScopedProofMode, error) {
	if m == nil {
		return nil, ErrASTHandle
	}
	if err := m.lock(); err != nil {
		return nil, err
	}
	defer m.mu.Unlock()
	if mode > ASTProofEnabled {
		return nil, ErrASTDomain
	}
	s := &ScopedProofMode{manager: m, previous: m.proofMode}
	s.self = s
	m.proofMode = mode
	return s, nil
}

// NewScopedProofMode selects mode until the returned scope is closed.
func NewScopedProofMode(m *ASTManager, mode ASTProofMode) (*ScopedProofMode, error) {
	return newScopedProofMode(m, mode)
}

// NewScopedProof enables proof generation for the returned scope.
func NewScopedProof(m *ASTManager) (*ScopedProofMode, error) {
	return newScopedProofMode(m, ASTProofEnabled)
}

// NewScopedNoProof disables proof generation for the returned scope.
func NewScopedNoProof(m *ASTManager) (*ScopedProofMode, error) {
	return newScopedProofMode(m, ASTProofDisabled)
}

// NewScopedRestoreProof records the current mode without changing it. Closing
// the scope restores that mode after changes made inside the scope.
func NewScopedRestoreProof(m *ASTManager) (*ScopedProofMode, error) {
	if m == nil {
		return nil, ErrASTHandle
	}
	if err := m.lock(); err != nil {
		return nil, err
	}
	defer m.mu.Unlock()
	s := &ScopedProofMode{manager: m, previous: m.proofMode}
	s.self = s
	return s, nil
}

func (s *ScopedProofMode) Close() error {
	if s == nil || s.self != s || s.manager == nil {
		return ErrASTHandle
	}
	if s.closed {
		return nil
	}
	if err := s.manager.lock(); err != nil {
		return err
	}
	s.manager.proofMode = s.previous
	s.manager.mu.Unlock()
	s.closed = true
	return nil
}
