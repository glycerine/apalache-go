package ast

// A view borrows the collector's references in insertion order. The collector
// drops its current view whenever keys change, invalidating outstanding iterators
// before any freed key can be returned. Copies share the same view identity.
type astSharedView struct{ nodes []astNodeRef }
type ASTSharedOccurrencesIterator struct {
	owner *ASTSharedOccurrences
	view  *astSharedView
	index int
}

func (s *ASTSharedOccurrences) BeginShared() (ASTSharedOccurrencesIterator, error) {
	m, e := s.owner.lock()
	if e != nil {
		return ASTSharedOccurrencesIterator{}, e
	}
	defer m.mu.Unlock()
	if s.resetPending {
		return ASTSharedOccurrencesIterator{}, ErrASTPending
	}
	if s.view == nil {
		view := &astSharedView{nodes: make([]astNodeRef, 0, s.entries.Len())}
		for _, entry := range s.entries.All() {
			view.nodes = append(view.nodes, entry.node)
		}
		s.view = view
	}
	return ASTSharedOccurrencesIterator{owner: s, view: s.view}, nil
}
func (s *ASTSharedOccurrences) EndShared() (ASTSharedOccurrencesIterator, error) {
	it, e := s.BeginShared()
	if e != nil {
		return it, e
	}
	it.index = len(it.view.nodes)
	return it, nil
}
func (it *ASTSharedOccurrencesIterator) check() error {
	if it == nil || it.owner == nil || it.view == nil {
		return ErrASTHandle
	}
	if e := it.owner.owner.check(); e != nil {
		return e
	}
	if it.owner.owner.manager.Closed || it.owner.view != it.view || it.owner.resetPending {
		return ErrASTHandle
	}
	return nil
}
func (it *ASTSharedOccurrencesIterator) Done() (bool, error) {
	if e := it.check(); e != nil {
		return false, e
	}
	return it.index == len(it.view.nodes), nil
}
func (it *ASTSharedOccurrencesIterator) Value() (ASTHandle, error) {
	if e := it.check(); e != nil {
		return ASTHandle{}, e
	}
	if it.index >= len(it.view.nodes) {
		return ASTHandle{}, ErrASTHandle
	}
	return it.owner.owner.manager.external(it.view.nodes[it.index]), nil
}
func (it *ASTSharedOccurrencesIterator) Next() error {
	if e := it.check(); e != nil {
		return e
	}
	if it.index >= len(it.view.nodes) {
		return ErrASTHandle
	}
	it.index++
	return nil
}
func (it *ASTSharedOccurrencesIterator) Equal(other ASTSharedOccurrencesIterator) (bool, error) {
	if e := it.check(); e != nil {
		return false, e
	}
	if e := other.check(); e != nil {
		return false, e
	}
	return it.owner == other.owner && it.view == other.view && it.index == other.index, nil
}
