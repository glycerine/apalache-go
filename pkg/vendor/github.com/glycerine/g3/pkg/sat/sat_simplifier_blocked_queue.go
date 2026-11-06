package sat

// Source: src/sat/sat_simplifier.cpp:875–919.

func newSATSimplifierBlockedQueue(s *satSimplifier) (*satVarQueue, error) {
	if s == nil || s.self != s {
		return nil, ErrSATOwner
	}
	if s.lifecycle == nil || s.lifecycle.propagation == nil || s.lifecycle.propagation.memory == nil || s.lifecycle.budget != s.lifecycle.propagation.memory.Budget() {
		return nil, ErrSATOwner
	}
	q, e := newSATVarQueueWithDebug(s.lifecycle.budget, s.debug)
	if e != nil {
		return nil, e
	}
	q.literalSource = s
	q.initialize()
	if e := q.cassertInvariant(); e != nil {
		_ = q.Close()
		return nil, e
	}
	if e := q.cassertInvariant(); e != nil {
		_ = q.Close()
		return nil, e
	}
	return q, nil
}
func (s *satSimplifier) literalWeight(idx uint32) (uint32, error) {
	u, e := s.useList.Get(Literal(idx).Negated())
	if e != nil {
		return 0, e
	}
	n, e := u.Size()
	if e != nil {
		return 0, e
	}
	w, e := s.lifecycle.propagation.variables.watches.Get(idx)
	if e != nil {
		return 0, e
	}
	count, e := w.Size()
	if e != nil {
		return 0, e
	}
	return 2*n + count, nil // source unsigned32 multiplication/addition wraps
}
func (s *satSimplifier) blockedDecreased(q *satVarQueue, l Literal) error {
	v := BoolVar(uint32(l))
	yes, e := q.containsChecked(v)
	if e != nil {
		return e
	}
	if yes {
		return q.ActivityIncreased(v)
	}
	return q.insert(v)
}
