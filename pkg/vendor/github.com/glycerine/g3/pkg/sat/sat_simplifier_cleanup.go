package sat

// Source: src/sat/sat_simplifier.cpp:206–318.

import (
	"github.com/glycerine/g3/pkg/arena"
)

func (s *satSimplifier) cleanupWatches() error {
	ws := s.lifecycle.propagation.variables.watches
	n := uint32(len(ws.lists))
	for row := uint32(0); row < n; row++ {
		w, e := ws.Get(row)
		if e != nil {
			return e
		}
		it, e := s.watchIterator(w)
		if e != nil {
			return e
		}
		out := uint32(0)
		for i := uint32(0); i < it.end; i++ {
			x, e := satPropagationCursorRead(&it, i)
			if e != nil {
				return e
			}
			switch x.Kind() {
			case 1, 2:
			default:
				if e = satPropagationCursorWrite(&it, out, x); e != nil {
					return e
				}
				out++
			}
		}
		if e = s.lifecycle.propagation.finishPropagationList(&it, w, out); e != nil {
			return e
		}
	}
	return nil
}
func (s *satSimplifier) beginCleanupClauses(learned bool) error {
	if s.lifecycle.simplifierContinuation != nil && s.lifecycle.simplifierContinuation != s {
		return ErrSATOwner
	}
	s.cleanupLearned = learned
	s.cleanupIndex = 0
	s.cleanupOut = 0
	s.cleanupEnd = uint32(len(s.clauseVector(learned)))
	s.drainPrefix = 0
	s.drainSuffix = 0
	s.cleanupInterrupted = true
	s.lifecycle.simplifierContinuation = s
	return nil
}
func (s *satSimplifier) finishCleanupClauses() {
	if s.cleanupLearned {
		s.lifecycle.learned = s.lifecycle.learned[:s.cleanupOut]
	} else {
		s.lifecycle.Original = s.lifecycle.Original[:s.cleanupOut]
	}
	s.cleanupInterrupted = false
	if !s.scopeActive {
		s.lifecycle.simplifierContinuation = nil
	}
}
func (s *satSimplifier) deleteCleanupClause(c StoredClauseRef) error {
	if e := s.lifecycle.delClause(c); e != nil {
		return e
	}
	s.cleanupIndex++
	return nil
}
func (s *satSimplifier) cleanupClauses(learned, varsEliminated, inUse bool) error {
	s.traceCleanupClauses()
	if e := s.beginCleanupClauses(learned); e != nil {
		return e
	}
	p := s.lifecycle.propagation
	a := p.clauses
	for s.cleanupIndex < s.cleanupEnd {
		vec := s.clauseVector(learned)
		if uint64(s.cleanupIndex) >= uint64(len(vec)) {
			return arena.ErrBounds
		}
		c := vec[s.cleanupIndex]
		removed, e := a.Removed(c)
		if e != nil {
			return e
		}
		if removed {
			if e = s.deleteCleanupClause(c); e != nil {
				return e
			}
			continue
		}
		if learned && varsEliminated {
			n, e := a.Size(c)
			if e != nil {
				return e
			}
			i := uint32(0)
			for ; i < n; i++ {
				l, e := a.Literal(c, i)
				if e != nil {
					return e
				}
				dead, e := p.variables.wasEliminated(l.variable())
				if e != nil {
					return e
				}
				if dead {
					break
				}
			}
			if i < n {
				if e = s.deleteCleanupClause(c); e != nil {
					return e
				}
				continue
			}
		}
		satisfied, e := s.cleanupClause(c, inUse)
		if e != nil {
			return e
		}
		if satisfied {
			if e = s.deleteCleanupClause(c); e != nil {
				return e
			}
			continue
		}
		n, e := a.Size(c)
		if e != nil {
			return e
		}
		switch n {
		case 0:
			p.setConflict(satJustification{}, DefaultLiteral())
			for s.cleanupIndex < s.cleanupEnd {
				vec = s.clauseVector(learned)
				vec[s.cleanupOut] = vec[s.cleanupIndex]
				s.cleanupOut++
				s.cleanupIndex++
			}
			s.finishCleanupClauses()
			return nil
		case 1:
			l, e := a.Literal(c, 0)
			if e != nil {
				return e
			}
			if e = p.assign(l, satJustification{}); e != nil {
				return e
			}
			if e = s.deleteCleanupClause(c); e != nil {
				return e
			}
			continue
		case 2:
			l1, e := a.Literal(c, 0)
			if e != nil {
				return e
			}
			l2, e := a.Literal(c, 1)
			if e != nil {
				return e
			}
			learned, e := a.Learned(c)
			if e != nil {
				return e
			}
			if e = s.lifecycle.mkBinClause(l1, l2, learned); e != nil {
				return e
			}
			if e = s.deleteCleanupClause(c); e != nil {
				return e
			}
			continue
		}
		if learned {
			wasLearned, e := a.Learned(c)
			if e != nil {
				return e
			}
			if !wasLearned {
				if satStorageSourceDebug {
					f, e := a.Frozen(c)
					if e != nil {
						return e
					}
					if f {
						return ErrSATAssertion
					}
				}
				if e = s.lifecycle.pushClause(false, c); e != nil {
					return e
				}
				s.cleanupIndex++
				continue
			}
		}
		vec = s.clauseVector(learned)
		vec[s.cleanupOut] = vec[s.cleanupIndex]
		s.cleanupOut++
		// Publication into the kept owning prefix occurs before the actual attach.
		// Consume the corresponding old-slot borrow before any refusal may unwind.
		s.cleanupIndex++
		f, e := a.Frozen(c)
		if e != nil {
			return e
		}
		if !f {
			if n == 3 {
				_, e = s.lifecycle.attachTerClause(c)
			} else {
				_, e = s.lifecycle.attachNaryClause(c)
			}
			if e != nil {
				return e
			}
		}
	}
	s.finishCleanupClauses()
	return nil
}
func (s *satSimplifier) scopedFinalize() error {
	if !s.scopeActive {
		return nil
	}
	// Source scoped destructor is never restarted after its captured pointer
	// compaction is interrupted. The owning Close adapter drains exact ranges.
	if s.cleanupInterrupted {
		return arena.ErrClosed
	}
	previous := s.running
	s.running = true
	defer func() { s.running = previous }()
	eliminated := s.numElimVars > s.oldNumElimVars
	// No completed destructor phase is replayed after a later checked refusal.
	for s.scopedStage < 5 {
		var e error
		switch s.scopedStage {
		case 0:
			s.traceFinalizeCleanupBranch()
			if s.needCleanup {
				e = s.cleanupWatches()
			}
		case 1:
			if s.needCleanup {
				e = s.cleanupClauses(true, eliminated, s.learnedInUseLists)
			} else if eliminated {
				e = s.cleanupClauses(true, true, s.learnedInUseLists)
			}
		case 2:
			if s.needCleanup {
				e = s.cleanupClauses(false, eliminated, true)
			}
		case 3:
			e = s.invariant()
			if e == nil {
				s.traceFinalizeDisplays()
			}
		case 4:
			e = s.Finalize()
		}
		if e != nil {
			return e
		}
		s.scopedStage++
	}
	s.scopeActive = false
	if s.lifecycle.simplifierContinuation == s {
		s.lifecycle.simplifierContinuation = nil
	}
	return nil
}

// Owning abort consumes actual kept-prefix and unread-suffix owners. Removed
// scalar duplicates and already consumed slots are not registry fallback roots.
func (s *satSimplifier) cleanupStopped() error {
	if !s.cleanupInterrupted {
		if s.scopeActive {
			// Owning lifecycle Close abandons unfinished source cleanup/diagnostics;
			// its ordinary real clause owners remain in their published vectors.
			if e := s.Finalize(); e != nil {
				return e
			}
			s.scopeActive = false
		}
		if s.lifecycle.simplifierContinuation == s {
			s.lifecycle.simplifierContinuation = nil
		}
		return nil
	}
	if s.lifecycle.simplifierContinuation != s {
		return ErrSATOwner
	}
	vec := s.clauseVector(s.cleanupLearned)
	if uint32(len(vec)) < s.cleanupEnd {
		return arena.ErrBounds
	}
	for s.drainPrefix < s.cleanupOut {
		if e := s.lifecycle.propagation.clauses.Delete(vec[s.drainPrefix]); e != nil {
			return e
		}
		s.drainPrefix++
	}
	if s.drainSuffix < s.cleanupIndex {
		s.drainSuffix = s.cleanupIndex
	}
	for s.drainSuffix < s.cleanupEnd {
		if e := s.lifecycle.propagation.clauses.Delete(vec[s.drainSuffix]); e != nil {
			return e
		}
		s.drainSuffix++
	}
	if s.cleanupLearned {
		s.lifecycle.learned = s.lifecycle.learned[:0]
	} else {
		s.lifecycle.Original = s.lifecycle.Original[:0]
	}
	s.cleanupInterrupted = false
	if !s.scopeActive {
		s.lifecycle.simplifierContinuation = nil
	}
	// Unsafe captured source compaction has been explicitly abandoned; only the
	// real remaining borrowed member storage is finalized by this owning adapter.
	if e := s.Finalize(); e != nil {
		return e
	}
	s.scopeActive = false
	if s.lifecycle.simplifierContinuation == s {
		s.lifecycle.simplifierContinuation = nil
	}
	return nil
}
