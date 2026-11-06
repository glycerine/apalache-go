package ast

// Source: src/ast/expr_map.cpp:24-94; src/ast/expr_map.h:32-60;
// src/util/dec_ref_util.h:24-64.

// ASTExprMap is the concrete expression-to-expression/proof map used by
// retained AST, API and Spacer consumers. The map owns one key and value
// reference per entry and, when enabled, one proof reference.
type ASTExprMap struct {
	self        *ASTExprMap
	manager     *ASTManager
	entries     map[astNodeRef]*astExprMapEntry
	pending     []astNodeRef
	storeProofs bool
	closed      bool
}

type astExprMapEntry struct {
	key, value, proof astNodeRef
	keyOwned          bool
	valueOwned        bool
	proofOwned        bool
}

// ASTExprMapPair is a borrowed key/value view for the retained API
// quantifier-elimination transfer. It does not own references.
type ASTExprMapPair struct {
	Key, Value ASTHandle
}

func NewASTExprMap(m *ASTManager) (*ASTExprMap, error) {
	if e := m.lock(); e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	s := &ASTExprMap{manager: m, storeProofs: m.proofMode != ASTProofDisabled, entries: make(map[astNodeRef]*astExprMapEntry)}
	s.self = s
	return s, nil
}

func NewASTExprMapWithProofs(m *ASTManager, storeProofs bool) (*ASTExprMap, error) {
	if e := m.lock(); e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	s := &ASTExprMap{manager: m, storeProofs: storeProofs, entries: make(map[astNodeRef]*astExprMapEntry)}
	s.self = s
	return s, nil
}

func (s *ASTExprMap) lock() (*ASTManager, error) {
	if s == nil || s.self != s || s.manager == nil {
		return nil, ErrASTHandle
	}
	m := s.manager
	if e := m.lock(); e != nil {
		return nil, e
	}
	if s.closed {
		m.mu.Unlock()
		return nil, ErrASTHandle
	}
	return m, nil
}

func (s *ASTExprMap) release(m *ASTManager, r astNodeRef) error {
	consumed, e := m.decNode(r)
	if !consumed {
		s.pending = append(s.pending, r)
	}
	return e
}

func (s *ASTExprMap) Insert(key, value, proof ASTHandle) error {
	m, e := s.lock()
	if e != nil {
		return e
	}
	defer m.mu.Unlock()
	k, e := m.requireExpr(key)
	if e != nil {
		return e
	}
	v, e := m.requireExpr(value)
	if e != nil {
		return e
	}
	var p astNodeRef
	if s.storeProofs && !proof.IsNull() {
		p, e = m.proofAppNode(proof)
		if e != nil {
			return e
		}
	}
	// Source INC of the incoming expression precedes replacement DEC, including
	// the self-replacement case.
	if e = m.incNode(v); e != nil {
		return e
	}
	entry, exists := s.entries[k]
	if !exists {
		if e = m.incNode(k); e != nil {
			if cleanup := s.release(m, v); cleanup != nil {
				return cleanup
			}
			return e
		}
		entry = &astExprMapEntry{key: k, value: v, keyOwned: true, valueOwned: true}
		s.entries[k] = entry
	} else {
		old := entry.value
		if e = s.release(m, old); e != nil {
			// Source still stores the old pointer when its DEC stops. The new
			// reference was already acquired, so the Go owner must retain it
			// for explicit cleanup without publishing it in the map.
			entry.valueOwned = false
			s.pending = append(s.pending, v)
			return e
		}
		entry.value = v
		entry.valueOwned = true
	}
	if s.storeProofs {
		if p.slot != 0 {
			if e = m.incNode(p); e != nil {
				return e
			}
		}
		old := entry.proof
		if old.slot != 0 {
			if e = s.release(m, old); e != nil {
				entry.proofOwned = false
				if p.slot != 0 {
					s.pending = append(s.pending, p)
				}
				return e
			}
		}
		entry.proof = p
		entry.proofOwned = p.slot != 0
	}
	return nil
}

func (s *ASTExprMap) Get(key ASTHandle) (value, proof ASTHandle, found bool, err error) {
	m, e := s.lock()
	if e != nil {
		return value, proof, false, e
	}
	defer m.mu.Unlock()
	k, e := m.requireExpr(key)
	if e != nil {
		return value, proof, false, e
	}
	entry, found := s.entries[k]
	if !found {
		return ASTHandle{}, ASTHandle{}, false, nil
	}
	return m.external(entry.value), m.external(entry.proof), true, nil
}

func (s *ASTExprMap) Contains(key ASTHandle) (bool, error) {
	_, _, found, e := s.Get(key)
	return found, e
}

// Pairs snapshots the source begin/end iterator's expression map. Callers
// that retain a pair beyond this map's lifetime must increment both AST refs.
func (s *ASTExprMap) Pairs() ([]ASTExprMapPair, error) {
	m, e := s.lock()
	if e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	pairs := make([]ASTExprMapPair, 0, len(s.entries))
	for _, entry := range s.entries {
		pairs = append(pairs, ASTExprMapPair{Key: m.external(entry.key), Value: m.external(entry.value)})
	}
	return pairs, nil
}

func (s *ASTExprMap) releaseProof(m *ASTManager, entry *astExprMapEntry) error {
	if !entry.proofOwned {
		return nil
	}
	entry.proofOwned = false
	return s.release(m, entry.proof)
}

func (s *ASTExprMap) releaseKeyValue(m *ASTManager, entry *astExprMapEntry) error {
	if entry.keyOwned {
		entry.keyOwned = false
		if e := s.release(m, entry.key); e != nil {
			return e
		}
	}
	if entry.valueOwned {
		entry.valueOwned = false
		if e := s.release(m, entry.value); e != nil {
			return e
		}
	}
	return nil
}

func (s *ASTExprMap) queueOwned(entry *astExprMapEntry) {
	if entry.proofOwned {
		s.pending = append(s.pending, entry.proof)
		entry.proofOwned = false
	}
	if entry.keyOwned {
		s.pending = append(s.pending, entry.key)
		entry.keyOwned = false
	}
	if entry.valueOwned {
		s.pending = append(s.pending, entry.value)
		entry.valueOwned = false
	}
}

func (s *ASTExprMap) drainPending(m *ASTManager) error {
	for len(s.pending) != 0 {
		last := len(s.pending) - 1
		r := s.pending[last]
		consumed, e := m.decNode(r)
		if consumed {
			s.pending = s.pending[:last]
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ErrASTPending
		}
	}
	return nil
}

func (s *ASTExprMap) Erase(key ASTHandle) error {
	m, e := s.lock()
	if e != nil {
		return e
	}
	defer m.mu.Unlock()
	k, e := m.requireExpr(key)
	if e != nil {
		return e
	}
	entry, found := s.entries[k]
	if !found {
		return nil
	}
	// Source removes the map entry before releasing its references.
	delete(s.entries, k)
	if entry.valueOwned {
		entry.valueOwned = false
		if e = s.release(m, entry.value); e != nil {
			s.queueOwned(entry)
			return e
		}
	}
	if e = s.releaseProof(m, entry); e != nil {
		s.queueOwned(entry)
		return e
	}
	if entry.keyOwned {
		entry.keyOwned = false
		return s.release(m, entry.key)
	}
	return nil
}

func (s *ASTExprMap) resetLocked(m *ASTManager) error {
	// Source reset first releases all proof values, then each expression key
	// and value. Map traversal order itself is not a source contract.
	for _, entry := range s.entries {
		if e := s.releaseProof(m, entry); e != nil {
			return e
		}
	}
	for key, entry := range s.entries {
		if e := s.releaseKeyValue(m, entry); e != nil {
			return e
		}
		delete(s.entries, key)
	}
	return s.drainPending(m)
}

func (s *ASTExprMap) Reset() error {
	m, e := s.lock()
	if e != nil {
		return e
	}
	defer m.mu.Unlock()
	return s.resetLocked(m)
}

func (s *ASTExprMap) Flush() error {
	m, e := s.lock()
	if e != nil {
		return e
	}
	defer m.mu.Unlock()
	if e = s.resetLocked(m); e != nil {
		return e
	}
	s.entries = make(map[astNodeRef]*astExprMapEntry)
	return nil
}

func (s *ASTExprMap) SetStoreProofs(flag bool) error {
	m, e := s.lock()
	if e != nil {
		return e
	}
	defer m.mu.Unlock()
	if s.storeProofs != flag {
		if e = s.resetLocked(m); e != nil {
			return e
		}
		s.entries = make(map[astNodeRef]*astExprMapEntry)
		s.storeProofs = flag
	}
	return nil
}

func (s *ASTExprMap) Close() error {
	m, e := s.lock()
	if e != nil {
		return e
	}
	defer m.mu.Unlock()
	if e = s.resetLocked(m); e != nil {
		return e
	}
	s.entries = nil
	s.closed = true
	return nil
}
