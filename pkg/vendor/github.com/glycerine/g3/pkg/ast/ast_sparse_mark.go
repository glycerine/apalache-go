package ast

import "reflect"

// ASTExprSparseMark borrows nodes, just like expr_sparse_mark.
type ASTExprSparseMark struct {
	owner   astMarkOwner
	entries map[astNodeRef]struct{}
}

func NewASTExprSparseMark(m *ASTManager) (*ASTExprSparseMark, error) {
	if e := m.lock(); e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	bytes := uint64(reflect.TypeOf(ASTExprSparseMark{}).Size())
	id, e := prepareASTMarkOwner(m, bytes)
	if e != nil {
		return nil, e
	}
	s := &ASTExprSparseMark{entries: make(map[astNodeRef]struct{})}
	s.owner.bind(m, bytes, id)
	return s, nil
}
func (s *ASTExprSparseMark) IsMarked(h ASTHandle) (bool, error) {
	m, e := s.owner.lock()
	if e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return false, e
	}
	_, yes := s.entries[r]
	return yes, nil
}
func (s *ASTExprSparseMark) Mark(h ASTHandle, flag bool) error {
	m, e := s.owner.lock()
	if e != nil {
		return e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return e
	}
	if flag {
		s.entries[r] = struct{}{}
		return nil
	}
	delete(s.entries, r)
	return nil
}
func (s *ASTExprSparseMark) Reset() error {
	m, e := s.owner.lock()
	if e != nil {
		return e
	}
	defer m.mu.Unlock()
	clear(s.entries)
	return nil
}
func (s *ASTExprSparseMark) Close() error {
	if e := s.owner.check(); e != nil {
		return e
	}
	m := s.owner.manager
	m.mu.Lock()
	defer m.mu.Unlock()
	s.entries = nil
	s.owner.finish()
	return nil
}
