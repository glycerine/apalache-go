package smtcore

// Source: src/smt/dyn_ack.cpp:118-178,216-294,315-389;
// src/util/obj_pair_hashtable.h.

import (
	"math"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

type dynAckPair struct{ first, second ast.ASTHandle }
type dynAckPairRecord struct {
	key        dynAckPair
	ownedStart int
}

// dynAckPairState is the concrete occurrence map and owning app vector used by
// dynamic Ackermannization. SMT context clause construction remains separate.
type dynAckPairState struct {
	manager       *ast.ASTManager
	threshold     uint32
	decay         float64
	counts        map[dynAckPair]uint32
	apps          []dynAckPairRecord
	toInstantiate []dynAckPair
	queueHead     int
	instantiated  map[dynAckPair]struct{}
	owned         []ast.ASTHandle
	closed        bool
}

func newDynAckPairState(m *ast.ASTManager, threshold uint32, decay float64) (*dynAckPairState, error) {
	if m == nil || math.IsNaN(decay) || decay < 0 || decay > 1 {
		return nil, ast.ErrASTDomain
	}
	return &dynAckPairState{
		manager: m, threshold: threshold, decay: decay,
		counts: make(map[dynAckPair]uint32), instantiated: make(map[dynAckPair]struct{}),
	}, nil
}

func (s *dynAckPairState) check() error {
	if s == nil || s.manager == nil || s.closed {
		return ast.ErrASTHandle
	}
	return nil
}

// congruenceEvent follows cg_eh: only distinct apps with the same declaration
// are presented by the SMT context, and basic equality apps are ignored.
func (s *dynAckPairState) congruenceEvent(n1, n2 ast.ASTHandle) error {
	if e := s.check(); e != nil {
		return e
	}
	m := s.manager
	for _, h := range []ast.ASTHandle{n1, n2} {
		kind, e := m.GetKind(h)
		if e != nil {
			return e
		}
		if kind != ast.ASTApp {
			return ast.ErrASTDomain
		}
	}
	if n1 == n2 {
		return ast.ErrASTDomain
	}
	d1, e := m.GetDecl(n1)
	if e != nil {
		return e
	}
	d2, e := m.GetDecl(n2)
	if e != nil {
		return e
	}
	if d1 != d2 {
		return ast.ErrASTDomain
	}
	family, e := m.GetFamilyID(d1)
	if e != nil {
		return e
	}
	kind, e := m.GetDeclKind(d1)
	if e != nil {
		return e
	}
	if family == m.BasicFamilyID() && kind == ast.ASTOpEq {
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
	key := dynAckPair{n1, n2}
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
		for _, h := range []ast.ASTHandle{n1, n2} {
			if e := m.IncRef(h); e != nil {
				return e
			}
			s.owned = append(s.owned, h)
		}
		s.apps = append(s.apps, dynAckPairRecord{key, start})
	}
	s.counts[key] = count
	if count == s.threshold {
		s.toInstantiate = append(s.toInstantiate, key)
	}
	return nil
}

func (s *dynAckPairState) markInstantiated(key dynAckPair) error {
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

func (s *dynAckPairState) clauseDeleted(key dynAckPair) error {
	if e := s.check(); e != nil {
		return e
	}
	delete(s.instantiated, key)
	return nil
}

func (s *dynAckPairState) releaseRecord(rec dynAckPairRecord) error {
	for i := 0; i < 2; i++ {
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

// gc is the live propagation path. Candidates follow decayed count descending,
// with source app-row order preserved when counts tie.
func (s *dynAckPairState) gc() error {
	if e := s.check(); e != nil {
		return e
	}
	keep := make([]dynAckPairRecord, 0, len(s.apps))
	decayed := make(map[dynAckPair]uint32, len(s.counts))
	for key, count := range s.counts {
		decayed[key] = count
	}
	candidates := make([]dynAckPair, 0, len(s.toInstantiate))
	for _, rec := range s.apps {
		if _, found := s.instantiated[rec.key]; found {
			if e := s.releaseRecord(rec); e != nil {
				return e
			}
			continue
		}
		count := uint32(float64(decayed[rec.key]) * s.decay)
		if count <= 1 {
			if e := s.releaseRecord(rec); e != nil {
				return e
			}
			delete(decayed, rec.key)
			continue
		}
		keep = append(keep, rec)
		decayed[rec.key] = count
		if count >= s.threshold {
			candidates = append(candidates, rec.key)
		}
	}
	s.apps = keep
	s.counts = decayed
	s.toInstantiate = candidates
	s.queueHead = 0
	sort.SliceStable(s.toInstantiate, func(i, j int) bool {
		return decayed[s.toInstantiate[i]] > decayed[s.toInstantiate[j]]
	})
	return nil
}

// initSearch releases app roots but retains live clause-instantiation records.
func (s *dynAckPairState) initSearch() error {
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

func (s *dynAckPairState) reset() error {
	if e := s.initSearch(); e != nil {
		return e
	}
	clear(s.instantiated)
	return nil
}

func (s *dynAckPairState) close() error {
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
