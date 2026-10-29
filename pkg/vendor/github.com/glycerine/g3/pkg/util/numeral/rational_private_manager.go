package numeral

// Source: src/util/mpq.h unsynch_mpq_manager construction used by
// src/ast/arith_decl_plugin.cpp:23-31.

import "sync/atomic"

// NewPrivateRationalManager constructs an independent unsynchronized rational
// manager in the service's allocator namespace. The returned manager borrows
// the service runtime and must be closed before the service.
func (s *RationalService) NewPrivateRationalManager() (*RationalManager, error) {
	if e := s.lock(); e != nil {
		return nil, e
	}
	defer s.mu.Unlock()
	c := s.initializationConfig.Integers
	c.Memory = s.Manager.RuntimeOwner()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	return NewRationalManager(RationalConfig{Integers: c, Synchronized: false})
}
