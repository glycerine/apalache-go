package ast

import "reflect"

// Visitation borrows nodes; native slice growth has no object-arena charge.
type astSharedMarkState struct{ nodes []astNodeRef }

func (s *astSharedMarkState) mark(m *ASTManager, r astNodeRef, flag bool) error {
	if e := m.validNode(r); e != nil {
		return e
	}
	n := m.nodeAt(r.slot)
	if !flag {
		n.shared = false
		return nil
	}
	if n.shared {
		return nil
	}
	n.shared = true
	s.nodes = append(s.nodes, r)
	return nil
}
func (s *astSharedMarkState) reset(m *ASTManager) error {
	for _, r := range s.nodes {
		if e := m.validNode(r); e != nil {
			return e
		}
		m.nodeAt(r.slot).shared = false
	}
	s.nodes = s.nodes[:0]
	return nil
}
func (s *astSharedMarkState) close(m *ASTManager) error {
	if !m.Closed {
		if e := s.reset(m); e != nil {
			return e
		}
	}
	s.nodes = nil
	return nil
}

type ASTSharedOccurrencesMark struct {
	owner astMarkOwner
	state astSharedMarkState
}

func NewASTSharedOccurrencesMark(m *ASTManager) (*ASTSharedOccurrencesMark, error) {
	if e := m.lock(); e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	bytes := uint64(reflect.TypeOf(ASTSharedOccurrencesMark{}).Size())
	id, e := prepareASTMarkOwner(m, bytes)
	if e != nil {
		return nil, e
	}
	s := &ASTSharedOccurrencesMark{}
	s.owner.bind(m, bytes, id)
	return s, nil
}
func (s *ASTSharedOccurrencesMark) IsMarked(h ASTHandle) (bool, error) {
	m, e := s.owner.lock()
	if e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return false, e
	}
	return m.nodeAt(r.slot).shared, nil
}
func (s *ASTSharedOccurrencesMark) Mark(h ASTHandle, flag bool) error {
	m, e := s.owner.lock()
	if e != nil {
		return e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return e
	}
	return s.state.mark(m, r, flag)
}
func (s *ASTSharedOccurrencesMark) ResetMark(h ASTHandle) error { return s.Mark(h, false) }
func (s *ASTSharedOccurrencesMark) Reset() error {
	m, e := s.owner.lock()
	if e != nil {
		return e
	}
	defer m.mu.Unlock()
	return s.state.reset(m)
}
func (s *ASTSharedOccurrencesMark) Close() error {
	if e := s.owner.check(); e != nil {
		return e
	}
	m := s.owner.manager
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := s.state.close(m); e != nil {
		return e
	}
	s.owner.finish()
	return nil
}
