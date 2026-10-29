package sat

// Source: src/sat/sat_probing.cpp; src/sat/sat_probing.h.

import (
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/stopwatch"
)

func (s *satSolverProbing) tryLit(l Literal, updateCache bool) (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	if e := s.assertReady(l, 61, "s.value(l.var()) == l_undef"); e != nil {
		return false, e
	}
	var implied satProbingCacheBorrow
	var found bool
	var e error
	if !updateCache {
		implied, found, e = s.cachedImpliedLits(l)
		if e != nil {
			return false, e
		}
	}
	p := s.propagation()
	v := p.variables
	if found {
		block, end, e := implied.capture()
		if e != nil {
			return false, e
		}
		for i := uint32(0); i < end; i++ {
			x, e := implied.readCaptured(block, i)
			if e != nil {
				return false, e
			}
			member := s.assigned.contains(x)
			if member {
				// Source dereferences the captured iterator again for assign.
				x, e = implied.readCaptured(block, i)
				if e != nil {
					return false, e
				}
				if e = p.assign(x, satJustification{}); e != nil {
					return false, e
				}
				s.numAssigned++
			}
		}
	} else {
		if e = s.toAssert.Shrink(0); e != nil {
			return false, e
		}
		if e = s.lifecycle.push(); e != nil {
			return false, e
		}
		if e = p.assign(l, satJustification{}); e != nil {
			return false, e
		}
		if e = s.decrement(); e != nil {
			return false, e
		}
		old, e := v.trail.Size(v.trailRef)
		if e != nil {
			return false, e
		}
		if e = s.propagate(); e != nil {
			return false, e
		}
		if p.inconsistent {
			if e = s.lifecycle.pop(1); e != nil {
				return false, e
			}
			if e = p.assign(l.Negated(), satJustification{}); e != nil {
				return false, e
			}
			e = s.propagate()
			return false, e
		}
		end, e := v.trail.Size(v.trailRef)
		if e != nil {
			return false, e
		}
		for i := old; i < end; i++ {
			x, e := v.trailRead(i)
			if e != nil {
				return false, e
			}
			member := s.assigned.contains(x)
			if member {
				x, e = v.trailRead(i)
				if e != nil {
					return false, e
				}
				if e = s.toAssert.Push(uint32(x)); e != nil {
					return false, e
				}
			}
		}
		if updateCache {
			if e = s.cacheBins(l, old); e != nil {
				return false, e
			}
		}
		if e = s.lifecycle.pop(1); e != nil {
			return false, e
		}
		end, e = s.toAssert.Size()
		if e != nil {
			return false, e
		}
		values := s.toAssert.values
		for i := uint32(0); i < end; i++ {
			x := values[i]
			if e = p.assign(Literal(x), satJustification{}); e != nil {
				return false, e
			}
			s.numAssigned++
		}
	}
	if e = s.propagate(); e != nil {
		return false, e
	}
	return !p.inconsistent, nil
}
func (s *satSolverProbing) processCore(id BoolVar) error {
	if e := s.open(); e != nil {
		return e
	}
	s.traceProcessing(id)
	l := MakeLiteral(id, false)
	if e := s.assertReady(l, 112, "s.value(v) == l_undef"); e != nil {
		return e
	}
	if e := s.decrement(); e != nil {
		return e
	}
	if e := s.lifecycle.push(); e != nil {
		return e
	}
	p := s.propagation()
	v := p.variables
	if e := p.assign(l, satJustification{}); e != nil {
		return e
	}
	old, e := v.trail.Size(v.trailRef)
	if e != nil {
		return e
	}
	if e = s.propagate(); e != nil {
		return e
	}
	if p.inconsistent {
		if e = s.lifecycle.pop(1); e != nil {
			return e
		}
		if e = p.assign(l.Negated(), satJustification{}); e != nil {
			return e
		}
		if e = s.propagate(); e != nil {
			return e
		}
		s.numAssigned++
		return nil
	}
	s.assigned.Reset()
	end, e := v.trail.Size(v.trailRef)
	if e != nil {
		return e
	}
	for i := old; i < end; i++ {
		x, e := v.trailRead(i)
		if e != nil {
			return e
		}
		if e = s.assigned.Insert(x); e != nil {
			return e
		}
	}
	if e = s.cacheBins(l, old); e != nil {
		return e
	}
	if e = s.lifecycle.pop(1); e != nil {
		return e
	}
	keep, e := s.tryLit(l.Negated(), true)
	if e != nil {
		return e
	}
	if !keep {
		return nil
	}
	if s.binaryEnabled {
		list, e := v.watches.Get(l.Negated().index())
		if e != nil {
			return e
		}
		// Source index loop rereads current size/item as try_lit can mutate watches.
		for i := uint32(0); ; i++ {
			n, e := list.Size()
			if e != nil {
				return e
			}
			if i >= n {
				break
			}
			w, e := list.Read(i)
			if e != nil {
				return e
			}
			if !w.Binary() {
				break
			}
			other, e := w.Literal()
			if e != nil {
				return e
			}
			if l.index() > other.index() {
				continue
			}
			value, e := v.valueLiteral(other)
			if e != nil {
				return e
			}
			if value != lbool.Undef {
				continue
			}
			keep, e := s.tryLit(other, false)
			if e != nil {
				return e
			}
			if !keep || p.inconsistent {
				return nil
			}
		}
	}
	return nil
}
func (s *satSolverProbing) process(id BoolVar) error {
	if e := s.open(); e != nil {
		return e
	}
	old := s.propagation().probingCounter
	assigned := s.numAssigned
	if e := s.processCore(id); e != nil {
		return e
	}
	if s.numAssigned > assigned {
		s.propagation().probingCounter = old
	}
	return nil
}
func (s *satSolverProbing) run(force bool) (answer bool, err error) {
	if e := s.open(); e != nil {
		return false, e
	}
	if !s.enabled {
		return true, nil
	}
	if e := s.propagate(); e != nil {
		return false, e
	}
	p := s.propagation()
	v := p.variables
	if p.inconsistent {
		return true, nil
	}
	if satStorageSourceDebug {
		if e := s.assertQHead(199); e != nil {
			return false, e
		}
		if e := s.assertConditionalInvariant(); e != nil {
			return false, e
		}
	}
	if !force && p.probingCounter > 0 {
		return true, nil
	}
	if s.cacheEnabled {
		a, e := p.memory.AllocationSize()
		if e != nil {
			return false, e
		}
		if a > s.cacheLimit {
			if e = s.cache.finalize(s); e != nil {
				return false, e
			}
		}
	}
	// Source lexical report starts here and stops on every normal/error unwind.
	oldAssigned := s.numAssigned
	var watch stopwatch.Stopwatch
	if e := watch.Start(); e != nil {
		return false, e
	}
	defer func() {
		e := watch.Stop()
		if e == nil {
			e = s.emitReport(&watch, oldAssigned)
		}
		if e != nil && err == nil {
			answer = false
			err = e
		}
	}()
	result := true
	p.probingCounter = 0
	// uint32 to int32 is the selected source conversion. Negating INT_MIN is UB.
	if int32(s.limit) == -1<<31 {
		return false, ErrSATSignedCounterDomain
	}
	limit := -int32(s.limit)
	num, e := v.numVarsChecked()
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < num; i++ {
		id := (s.stoppedAt + i) % num
		if p.probingCounter < limit {
			s.stoppedAt = id
			result = false
			break
		}
		if p.inconsistent {
			break
		}
		value, e := v.valueVar(BoolVar(id))
		if e != nil {
			return false, e
		}
		skip := value != lbool.Undef
		if !skip {
			x, e := v.wasEliminated(BoolVar(id))
			if e != nil {
				return false, e
			}
			skip = x
		}
		if skip {
			if s.cacheEnabled {
				if e = s.resetCache(MakeLiteral(BoolVar(id), false)); e != nil {
					return false, e
				}
				if e = s.resetCache(MakeLiteral(BoolVar(id), true)); e != nil {
					return false, e
				}
			}
			continue
		}
		if e = p.checkpoint(); e != nil {
			return false, e
		}
		if e = s.process(BoolVar(id)); e != nil {
			return false, e
		}
	}
	if result {
		s.stoppedAt = 0
	}
	if p.probingCounter == -1<<31 {
		return false, ErrSATSignedCounterDomain
	}
	p.probingCounter = -p.probingCounter
	if oldAssigned == s.numAssigned {
		if p.probingCounter > 1<<30-1 || p.probingCounter < -1<<30 {
			return false, ErrSATSignedCounterDomain
		}
		p.probingCounter *= 2
	}
	if e := s.assertConditionalInvariant(); e != nil {
		return false, e
	}
	if e := s.finalize(); e != nil {
		return false, e
	}
	return result, nil
}
