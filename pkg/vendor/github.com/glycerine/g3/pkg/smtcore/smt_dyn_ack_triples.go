package smtcore

// Source: src/smt/dyn_ack.cpp:174-215,309-315,390-418;
// src/smt/dyn_ack.h:39-69; src/util/obj_triple_hashtable.h:21-177.

import (
	"math"

	"github.com/glycerine/g3/pkg/ast"
)

type dynAckTriple struct{ first, second, representative ast.ASTHandle }
type dynAckTripleRecord struct {
	key        dynAckTriple
	ownedStart int
}

// dynAckTripleState is the reached app-triple occurrence map/set and owning
// app vector. The enclosing SMT context still owns clause construction.
type dynAckTripleState struct {
	manager       *ast.ASTManager
	threshold     uint32
	counts        map[dynAckTriple]uint32
	apps          []dynAckTripleRecord
	toInstantiate []dynAckTriple
	queueHead     int
	instantiated  map[dynAckTriple]struct{}
	owned         []ast.ASTHandle
	closed        bool
}

func newDynAckTripleState(m *ast.ASTManager, threshold uint32) (*dynAckTripleState, error) {
	if m == nil {
		return nil, ast.ErrASTDomain
	}
	return &dynAckTripleState{
		manager: m, threshold: threshold,
		counts: make(map[dynAckTriple]uint32), instantiated: make(map[dynAckTriple]struct{}),
	}, nil
}

func (s *dynAckTripleState) check() error {
	if s == nil || s.manager == nil || s.closed {
		return ast.ErrASTHandle
	}
	return nil
}

// eqEvent is the source eq_eh triplet branch. The bool-sort and alias guards
// precede canonical ordering, occurrence accounting and threshold scheduling.
func (s *dynAckTripleState) eqEvent(n1, n2, representative ast.ASTHandle) error {
	if e := s.check(); e != nil {
		return e
	}
	m := s.manager
	for _, h := range []ast.ASTHandle{n1, n2, representative} {
		kind, e := m.GetKind(h)
		if e != nil {
			return e
		}
		if kind != ast.ASTApp {
			return ast.ErrASTDomain
		}
	}
	if n1 == n2 || representative == n1 || representative == n2 {
		return nil
	}
	sort1, e := m.GetSort(n1)
	if e != nil {
		return e
	}
	boolSort, e := m.MkBoolSort()
	if e != nil {
		return e
	}
	if sort1 == boolSort {
		return nil
	}
	id1, e := m.GetASTID(n1)
	if e != nil {
		return e
	}
	id2, e := m.GetASTID(n2)
	if e != nil {
		return e
	}
	if id1 > id2 {
		n1, n2 = n2, n1
	}
	key := dynAckTriple{n1, n2, representative}
	if _, found := s.instantiated[key]; found {
		return nil
	}
	count, found := s.counts[key]
	if found {
		if count == math.MaxUint32 {
			return ast.ErrASTDomain
		}
		count++
	} else {
		count = 1
		start := len(s.owned)
		for _, h := range []ast.ASTHandle{n1, n2, representative} {
			if e := m.IncRef(h); e != nil {
				return e // successful earlier pins remain reachable in owned
			}
			s.owned = append(s.owned, h)
		}
		s.apps = append(s.apps, dynAckTripleRecord{key, start})
	}
	s.counts[key] = count
	if count == s.threshold {
		s.toInstantiate = append(s.toInstantiate, key)
	}
	return nil
}

// markInstantiated records the source map-erase/set-insert transition after
// selecting a candidate. Clause construction and clause IDs belong to context.
func (s *dynAckTripleState) markInstantiated(key dynAckTriple) error {
	if e := s.check(); e != nil {
		return e
	}
	if _, found := s.counts[key]; !found {
		return ast.ErrASTDomain
	}
	delete(s.counts, key)
	s.instantiated[key] = struct{}{}
	return nil
}

func (s *dynAckTripleState) clauseDeleted(key dynAckTriple) error {
	if e := s.check(); e != nil {
		return e
	}
	delete(s.instantiated, key)
	return nil
}

func (s *dynAckTripleState) releaseRecord(rec dynAckTripleRecord) error {
	for i := 0; i < 3; i++ {
		at := rec.ownedStart + i
		if at >= len(s.owned) {
			return ast.ErrASTDomain
		}
		h := s.owned[at]
		if h.IsNull() {
			continue
		}
		consumed, e := s.manager.DecRefConsumed(h)
		if consumed {
			s.owned[at] = ast.ASTHandle{}
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	return nil
}

// initSearch follows init_search_eh for this triple state; reset additionally
// forgets instantiated clauses, as the enclosing source reset does.
func (s *dynAckTripleState) initSearch() error {
	if e := s.check(); e != nil {
		return e
	}
	for _, rec := range s.apps {
		if e := s.releaseRecord(rec); e != nil {
			return e
		}
	}
	s.apps = nil
	clear(s.counts)
	s.toInstantiate = nil
	s.queueHead = 0
	return nil
}

func (s *dynAckTripleState) reset() error {
	if e := s.initSearch(); e != nil {
		return e
	}
	clear(s.instantiated)
	return nil
}

func (s *dynAckTripleState) close() error {
	if s == nil || s.closed {
		return nil
	}
	for i, h := range s.owned {
		if h.IsNull() {
			continue
		}
		consumed, e := s.manager.DecRefConsumed(h)
		if consumed {
			s.owned[i] = ast.ASTHandle{}
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	s.owned = nil
	s.apps = nil
	s.toInstantiate = nil
	s.counts = nil
	s.instantiated = nil
	s.closed = true
	return nil
}
