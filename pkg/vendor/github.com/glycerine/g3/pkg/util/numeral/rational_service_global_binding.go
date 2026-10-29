package numeral

// Source: src/util/rational.cpp:64-83; src/util/rational.h:67;
// src/util/inf_rational.cpp:195-198; src/util/inf_int_rational.cpp:56-59.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

var ErrRationalStaticLifetime = errors.New("mpnum: source rational static object lifetime ended")

type RationalSourceStatic uint8

const (
	SourceStaticInfZero RationalSourceStatic = iota
	SourceStaticInfOne
	SourceStaticInfMinusOne
	SourceStaticInfIntZero
	SourceStaticInfIntOne
	SourceStaticInfIntMinusOne
	SourceStaticZero
	SourceStaticOne
	SourceStaticMinusOne
)

// This scalar slot is borrowed, never an additional manager ownership root.
// It must outlive this exact static context. Bind before constructing a manager;
// the namespace validates its actual slot address on every reached operation.
func (s *RationalService) BindSourceGlobalSlot(slot **RationalManager) error {
	if s == nil || s.self != s || slot == nil {
		return ErrRationalServiceOwner
	}
	if s.sourceInsertionExecuting || s.runtimeTransportExecuting() {
		return arena.ErrInvalidSlot
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.closing {
		return arena.ErrClosed
	}
	if s.sourceGlobalSlot != nil {
		if s.sourceGlobalSlot == slot {
			return nil
		}
		return ErrRationalServiceOwner
	}
	if s.Manager != nil || s.initializationStage != 0 || s.sourceFinalizing || *slot != nil {
		return ErrRationalServiceOwner
	}
	s.sourceGlobalSlot = slot
	return nil
}
func (s *RationalService) ValidateSourceGlobalSlot(slot **RationalManager) error {
	if s == nil || s.self != s || slot == nil || s.sourceGlobalSlot != slot {
		return ErrRationalServiceOwner
	}
	if s.closed || s.closing {
		return arena.ErrClosed
	}
	return nil
}
func (s *RationalService) SourceEndedStaticMask() (uint16, error) {
	if s == nil || s.self != s {
		return 0, ErrRationalServiceOwner
	}
	if s.sourceInsertionExecuting || s.runtimeTransportExecuting() {
		return 0, arena.ErrInvalidSlot
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sourceEndedStatics, nil // diagnostic cold history, including stopped teardown
}
func (s *RationalService) checkSourceStaticLifetimeLocked(which RationalSourceStatic) error {
	if which > SourceStaticMinusOne {
		return ErrRationalServiceOwner
	}
	if s.sourceGlobalSlot != nil && s.sourceEndedStatics&(uint16(1)<<which) != 0 {
		return ErrRationalStaticLifetime
	}
	return nil
}
func (s *RationalService) CheckSourceStaticLifetime(slot **RationalManager, which RationalSourceStatic) error {
	if e := s.ValidateSourceGlobalSlot(slot); e != nil {
		return e
	}
	if s.sourceInsertionExecuting || s.runtimeTransportExecuting() {
		return arena.ErrInvalidSlot
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.checkSourceStaticLifetimeLocked(which)
}

// Only actual completed owning closure detaches the borrowed scalar slot.
func (s *RationalService) DetachSourceGlobalSlot(slot **RationalManager) error {
	if s == nil || s.self != s || slot == nil || s.sourceGlobalSlot != slot || !s.closed {
		return ErrRationalServiceOwner
	}
	*slot = nil
	s.sourceGlobalSlot = nil
	return nil
}

// The namespace owns this newly constructed manager in its exact acquisition
// node; this method borrows it only for the literal constant initialization
// prefix. It never replaces the original retained static-context manager.
func (s *RationalService) InitializeReboundSourceConstants(slot **RationalManager, m *RationalManager) error {
	if e := s.ValidateSourceGlobalSlot(slot); e != nil {
		return e
	}
	if m == nil || *slot != m {
		return ErrRationalServiceOwner
	}
	if s.sourceInsertionExecuting || s.runtimeTransportExecuting() {
		return arena.ErrInvalidSlot
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.initializeConstantsSource(m)
}

// PublishAcquiredSourceManager transfers one actual completed constructor owner
// to the original prepared static context. No failed constructor is adopted.
// All guards precede the source pointer publication; success has no fallible
// action after that transfer. Constant initialization is the next source action.
func (s *RationalService) PublishAcquiredSourceManager(slot **RationalManager, m *RationalManager) error {
	if e := s.ValidateSourceGlobalSlot(slot); e != nil {
		return e
	}
	if s.sourceInsertionExecuting || s.runtimeTransportExecuting() {
		return arena.ErrInvalidSlot
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if m == nil || *slot != nil || s.Manager != nil || s.sourceFinalizing || s.initializationFailure != nil {
		return ErrRationalServiceOwner
	}
	s.Manager = m
	m.Z.rationalServiceParent = s
	*slot = m
	s.initializationStage = 1
	return nil
}
