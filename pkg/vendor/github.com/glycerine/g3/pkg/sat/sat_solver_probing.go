package sat

// Source: src/sat/sat_probing.cpp; src/sat/sat_probing.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
)

var ErrSATMissingIntegrityBinding = errors.New("g3: missing source SAT integrity-checker binding")

// This is the actual probing member family, not solver construction/search.
// Borrowed lifecycle, propagation, parameters and Runtime must outlive it.
type satSolverProbing struct {
	self                                 *satSolverProbing
	lifecycle                            *satSolverClauseLifecycle
	budget                               *arena.Budget
	assigned                             satLiteralSetStorage
	toAssert                             satUintColumn
	cacheWork                            satUintColumn // embedded workspace; native slices move back on return.
	cache                                satProbingCache
	stoppedAt, limit, numAssigned        uint32
	cacheLimit, ownerBytes               uint64
	enabled, cacheEnabled, binaryEnabled bool
	constructed, closing, closed         bool
}

// The cold header and bound empty vectors are explicit Go ownership context;
// actual source parameter/statistics construction remains initializeSource.
func newSATColdProbing(l *satSolverClauseLifecycle, b *arena.Budget) (*satSolverProbing, error) {
	if e := l.constructionBorrow(); e != nil {
		return nil, e
	}
	if b != l.budget || b != l.propagation.memory.Budget() {
		return nil, ErrSATOwner
	}
	n := uint64(reflect.TypeOf(satSolverProbing{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	s := &satSolverProbing{lifecycle: l, budget: b, ownerBytes: n}
	s.self = s
	s.toAssert.Bind(l.propagation.memory)
	s.cacheWork.Bind(l.propagation.memory)
	return s, nil
}
func (s *satSolverProbing) initializeSource(c *uparams.ParamsContext, v *uparams.ParamsRef) error {
	if e := s.constructionBorrow(); e != nil {
		return e
	}
	if s.constructed {
		return ErrSATOwner
	}
	if e := s.updateParams(c, v); e != nil {
		return e
	}
	s.resetStatistics()
	s.stoppedAt = 0
	s.propagation().probingCounter = 0
	s.constructed = true
	return nil
}
func newSATSolverProbing(l *satSolverClauseLifecycle, c *uparams.ParamsContext, v *uparams.ParamsRef, b *arena.Budget) (*satSolverProbing, error) {
	s, e := newSATColdProbing(l, b)
	if e != nil {
		return nil, e
	}
	if e = s.initializeSource(c, v); e != nil {
		if cleanup := s.Close(); cleanup != nil {
			return s, cleanup
		}
		return nil, e
	}
	return s, nil
}
func (s *satSolverProbing) propagation() *satSolverPropagationState { return s.lifecycle.propagation }
func (s *satSolverProbing) constructionBorrow() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closed || s.closing {
		return arena.ErrClosed
	}
	return s.lifecycle.constructionBorrow()
}
func (s *satSolverProbing) open() error {
	if e := s.constructionBorrow(); e != nil {
		return e
	}
	if !s.constructed {
		return arena.ErrClosed
	}
	return nil
}
func (s *satSolverProbing) updateParams(c *uparams.ParamsContext, p *uparams.ParamsRef) error {
	if e := s.constructionBorrow(); e != nil {
		return e
	}
	var e error
	k := "probing"
	s.enabled, e = c.GetBoolName(p, &k, true)
	if e != nil {
		return e
	}
	k = "probing_limit"
	s.limit, e = c.GetUIntName(p, &k, 5000000)
	if e != nil {
		return e
	}
	k = "probing_cache"
	s.cacheEnabled, e = c.GetBoolName(p, &k, true)
	if e != nil {
		return e
	}
	k = "probing_binary"
	s.binaryEnabled, e = c.GetBoolName(p, &k, true)
	if e != nil {
		return e
	}
	k = "probing_chache_limit"
	n, e := c.GetUIntName(p, &k, 1024)
	if e != nil {
		return e
	}
	s.cacheLimit = umemory.MegabytesToBytes(n)
	return nil
}
func (s *satSolverProbing) resetStatistics() { s.numAssigned = 0 }

func (s *satSolverProbing) collectStatistics(st *statistics.Statistics, key statistics.Key) error {
	if e := s.open(); e != nil {
		return e
	}
	st.UpdateUint(key, s.numAssigned)
	return nil
}

// Source collect_param_descrs has an empty TODO body.
func satProbingCollectParamDescriptions(d *uparams.ParamDescrs) {}
func (s *satSolverProbing) dec(n uint32) error {
	if e := s.open(); e != nil {
		return e
	}
	p := s.propagation()
	p.probingCounter = int32(uint32(p.probingCounter) - n)
	return nil
}
func (s *satSolverProbing) decrement() error {
	if s.propagation().probingCounter == -1<<31 {
		return ErrSATSignedCounterDomain
	}
	s.propagation().probingCounter--
	return nil
}
func (s *satSolverProbing) cacheBins(l Literal, old uint32) error {
	if e := s.open(); e != nil {
		return e
	}
	if !s.cacheEnabled {
		return nil
	}
	allocation, e := s.propagation().memory.AllocationSize()
	if e != nil {
		return e
	}
	if allocation > s.cacheLimit {
		return nil
	}
	if e = s.cache.reserve(s, l.index()+1); e != nil {
		return e
	}
	s.cache.rows[l.index()].available = true
	out, e := s.cache.workRow(s, l.index())
	if e != nil {
		return e
	}
	defer s.cache.saveWork(s, l.index())
	if e = out.Shrink(0); e != nil {
		return e
	}
	v := s.propagation().variables
	n, e := v.trail.Size(v.trailRef)
	if e != nil {
		return e
	}
	for i := old; i < n; i++ {
		x, e := v.trailRead(i)
		if e != nil {
			return e
		}
		if e = out.Push(uint32(x)); e != nil {
			return e
		}
	}
	return nil
}
func (s *satSolverProbing) cachedImpliedLits(l Literal) (satProbingCacheBorrow, bool, error) {
	if e := s.open(); e != nil {
		return satProbingCacheBorrow{}, false, e
	}
	if !s.cacheEnabled {
		return satProbingCacheBorrow{}, false, nil
	}
	n, e := s.cache.size(s)
	if e != nil {
		return satProbingCacheBorrow{}, false, e
	}
	if l.index() >= n {
		return satProbingCacheBorrow{}, false, nil
	}
	if !s.cache.rows[l.index()].available {
		return satProbingCacheBorrow{}, false, nil
	}
	return satProbingCacheBorrow{s, s.cache.epoch, l.index()}, true, nil
}
func (s *satSolverProbing) resetCache(l Literal) error {
	if e := s.open(); e != nil {
		return e
	}
	n, e := s.cache.size(s)
	if e != nil {
		return e
	}
	if l.index() >= n {
		return nil
	}
	s.cache.rows[l.index()].available = false
	out, e := s.cache.workRow(s, l.index())
	if e != nil {
		return e
	}
	e = out.Finalize()
	s.cache.saveWork(s, l.index())
	return e
}
func (s *satSolverProbing) finalize() error {
	s.assigned.Finalize()
	if e := s.toAssert.Finalize(); e != nil {
		return e
	}
	s.toAssert.Bind(s.propagation().memory)
	s.toAssert.closed = false
	return s.cache.finalize(s)
}
func (s *satSolverProbing) Close() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closed {
		return nil
	}
	s.closing = true
	// Reverse source member order: cached_bins, to_assert, assigned.
	if e := s.cache.finalize(s); e != nil {
		return e
	}
	if e := s.toAssert.Finalize(); e != nil {
		return e
	}
	s.assigned.Finalize()
	s.budget.Release(s.ownerBytes)
	s.ownerBytes = 0
	s.closed = true
	return nil
}

func (s *satSolverProbing) propagate() error {
	_, e := s.lifecycle.propagate(false)
	return e
}
