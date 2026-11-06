package numeral

// Source: src/util/rational.cpp:64-83; src/util/rational.h.
// Leonardo de Moura (2006-09-18).

import (
	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func (s *RationalService) Error() string { return "mpnum: retained rational service" }
func (s *RationalService) Unwrap() error {
	if s.originalCause != nil {
		return s.originalCause
	}
	return s.cleanupCause
}
func (s *RationalService) OriginalCause() error { return s.originalCause }
func (s *RationalService) CleanupCause() error  { return s.cleanupCause }
func (s *RationalService) RuntimeOwner() *umemory.Runtime {
	if s == nil || s.self != s {
		return nil
	}
	if s.Manager != nil {
		return s.Manager.RuntimeOwner()
	}
	if s.initializationFailure != nil {
		return s.initializationFailure.RuntimeOwner()
	}
	return nil
}
func (s *RationalService) Cleanup() error { return s.CloseChecked() }
func (s *RationalService) CloseChecked() error {
	if s == nil || s.self != s {
		return ErrRationalServiceCopy
	}
	if s.sourceInsertionExecuting || s.runtimeTransportExecuting() {
		return arena.ErrInvalidSlot
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		s.cleanupCause = nil
		return nil
	}
	if s.Manager == nil {
		s.closing = true
		if s.initializationFailure != nil {
			if e := s.initializationFailure.CloseChecked(); e != nil {
				s.cleanupCause = e
				return s
			}
			s.initializationFailure = nil
		}
		s.initializationBudget.Release(s.HeaderBytes)
		s.HeaderBytes = 0
		s.closed = true
		s.cleanupCause = nil
		return nil
	}
	if e := s.cleanupLexicalInvocations(); e != nil {
		s.cleanupCause = e
		return s
	}
	if e := s.Lexical.cleanup(s.Manager.Z); e != nil {
		s.cleanupCause = e
		return s
	}
	s.closing = true
	fail := func(e error) error { s.cleanupCause = e; return s }
	if s.sourceFinalizing && !s.sourceFinalized && !s.sourceGuard && !s.sourceGlobalStopped {
		if e := s.sourceFinalizeLocked(); e != nil {
			return fail(e)
		}
	}
	if !s.sourceGuard {
		if e := s.cleanupOuter(false); e != nil {
			return fail(e)
		}
	}
	if e := s.Manager.CloseChecked(); e != nil {
		return fail(e)
	}
	if s.sourceGuard {
		s.Cache = nil
		s.InfZeroField = OwnedInfRational{}
		s.InfOneField = OwnedInfRational{}
		s.InfMinusOneField = OwnedInfRational{}
		s.InfIntZeroField = OwnedInfIntRational{}
		s.InfIntOneField = OwnedInfIntRational{}
		s.InfIntMinusOneField = OwnedInfIntRational{}
		s.ZeroField = Rational{}
		s.OneField = Rational{}
		s.MinusOneField = Rational{}
	}
	s.Manager.BudgetField.Release(s.HeaderBytes)
	s.HeaderBytes = 0
	s.closed = true
	s.cleanupCause = nil
	return nil
}

func (s *RationalService) cleanupOuter(source bool) error {
	// rational.cpp finalization hooks, then ascending cache and constants.
	// inf_rational members destroy SECOND before FIRST, each NUM before DEN.
	infRoots := [18]*Int{
		&s.InfZeroField.Second.Num, &s.InfZeroField.Second.Den, &s.InfZeroField.First.Num, &s.InfZeroField.First.Den,
		&s.InfOneField.Second.Num, &s.InfOneField.Second.Den, &s.InfOneField.First.Num, &s.InfOneField.First.Den,
		&s.InfMinusOneField.Second.Num, &s.InfMinusOneField.Second.Den, &s.InfMinusOneField.First.Num, &s.InfMinusOneField.First.Den,
		&s.InfIntZeroField.First.Num, &s.InfIntZeroField.First.Den, &s.InfIntOneField.First.Num, &s.InfIntOneField.First.Den, &s.InfIntMinusOneField.First.Num, &s.InfIntMinusOneField.First.Den}
	for s.cleanupStage < 18 {
		if source && s.sourceGlobalSlot != nil {
			switch s.cleanupStage {
			case 0:
				s.sourceEndedStatics |= 1 << SourceStaticInfZero
			case 4:
				s.sourceEndedStatics |= 1 << SourceStaticInfOne
			case 8:
				s.sourceEndedStatics |= 1 << SourceStaticInfMinusOne
			case 12:
				s.sourceEndedStatics |= 1 << SourceStaticInfIntZero
			case 14:
				s.sourceEndedStatics |= 1 << SourceStaticInfIntOne
			case 16:
				s.sourceEndedStatics |= 1 << SourceStaticInfIntMinusOne
			}
		}
		if e := s.releaseOuterLeaf(infRoots[s.cleanupStage], source); e != nil {
			return e
		}
		s.cleanupStage++
		switch s.cleanupStage {
		case 4:
			s.InfZeroField = OwnedInfRational{}
		case 8:
			s.InfOneField = OwnedInfRational{}
		case 12:
			s.InfMinusOneField = OwnedInfRational{}
		case 14:
			s.InfIntZeroField = OwnedInfIntRational{}
		case 16:
			s.InfIntOneField = OwnedInfIntRational{}
		case 18:
			s.InfIntMinusOneField = OwnedInfIntRational{}
		}
	}
	if s.cleanupStage == 18 {
		for s.cleanupCacheIndex < uint32(len(s.Cache)) {
			r := &s.Cache[s.cleanupCacheIndex]
			p := &r.Num
			if s.cleanupComponent == 1 {
				p = &r.Den
			}
			if e := s.releaseOuterLeaf(p, source); e != nil {
				return e
			}
			s.cleanupComponent++
			if s.cleanupComponent == 2 {
				s.cleanupComponent = 0
				s.cleanupCacheIndex++
			}
		}
		s.Cache = nil
		s.cleanupStage++
	}
	constants := [6]*Int{&s.ZeroField.Num, &s.ZeroField.Den, &s.OneField.Num, &s.OneField.Den, &s.MinusOneField.Num, &s.MinusOneField.Den}
	for s.cleanupStage < 25 {
		if source && s.sourceGlobalSlot != nil {
			switch s.cleanupStage {
			case 19:
				s.sourceEndedStatics |= 1 << SourceStaticZero
			case 21:
				s.sourceEndedStatics |= 1 << SourceStaticOne
			case 23:
				s.sourceEndedStatics |= 1 << SourceStaticMinusOne
			}
		}
		if e := s.releaseOuterLeaf(constants[s.cleanupStage-19], source); e != nil {
			return e
		}
		s.cleanupStage++
	}

	return nil
}
