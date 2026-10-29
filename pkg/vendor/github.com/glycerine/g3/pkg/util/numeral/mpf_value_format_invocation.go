package numeral

// Source: src/util/mpf.cpp:1519-1569; src/util/mpq.cpp:156-185.
// rational display

import (
	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// ArbitraryFloatFormatInvocation is real cold continuation storage embedded in
// the exact text context. NUM fields acquire constructor credit only when their
// source declarations are reached; the text owner charges this actual record.

type ArbitraryFloatFormatInvocation struct {
	originalPending                               umemory.Block
	self                                          *ArbitraryFloatFormatInvocation
	manager                                       *ArbitraryFloatManager
	Next                                          *ArbitraryFloatFormatInvocation
	invocation                                    uint64
	active, executing, finishing, returned, owned bool
	Constructed                                   uint8
	sourceCursor, ownCursor                       uint8
	Num, Denom                                    Int
	Q                                             Rational
	stream                                        ArbitraryFloatFormatStream
	text                                          ArbitraryFloatFormatTarget
	Numeric                                       NumericAllocationRoots
	originalCell                                  SourceCellBacking
	cellRetired                                   bool
	Transferred                                   umemory.SmallAllocatorPendingAllocation
	gcd, division, extended                       integerRawInvocationReceipt

	Decimal                          rationalDecimalScope
	bodyCause, sourceCause, ownCause error
}

func (m *ArbitraryFloatManager) knownFormatFrame(s *ArbitraryFloatFormatInvocation) bool {
	if m == nil || m.self != m || s == nil {
		return false
	}
	for q := m.FormatScopes; q != nil; q = q.Next {
		if q == s {
			return true
		}
	}
	return false
}
func (m *ArbitraryFloatManager) MatchesFormatFrame(s *ArbitraryFloatFormatInvocation) bool {
	return m.knownFormatFrame(s) && s.self == s && s.manager == m && s.invocation != 0
}
func (m *ArbitraryFloatManager) formatInvocationExecuting() bool {
	if m == nil || m.self != m {
		return false
	}
	for s := m.FormatScopes; s != nil; s = s.Next {
		if m.MatchesFormatFrame(s) && (s.executing || s.finishing) {
			return true
		}
	}
	return false
}
func (m *ArbitraryFloatManager) beginFormatInvocation(s *ArbitraryFloatFormatInvocation, text ArbitraryFloatFormatTarget) error {
	if m == nil || m.self != m || m.integer == nil || m.integer.self != m.integer {
		return umemory.ErrRuntimeOwner
	}
	if m.closed || m.sourceEnded || !m.constructorComplete {
		return arena.ErrClosed
	}

	if s == nil || text == nil {
		return umemory.ErrRuntimeOwner
	}
	if m.formatInvocationExecuting() || m.valueExecuting {
		return ErrNumericPending
	}
	if s.self != nil || s.invocation != 0 || m.formatInvocation == ^uint64(0) {
		return ErrNumericPending
	}
	m.formatInvocation++
	*s = ArbitraryFloatFormatInvocation{self: s, manager: m, Next: m.FormatScopes, invocation: m.formatInvocation, active: true, executing: true, text: text}
	m.FormatScopes = s
	return nil
}

// FormatInvocationCauses is immutable invocation history, not acquisition.
func (m *ArbitraryFloatManager) FormatInvocationCauses(s *ArbitraryFloatFormatInvocation) (body, source, own error) {
	if !m.MatchesFormatFrame(s) {
		return umemory.ErrRuntimeOwner, nil, nil
	}
	return s.bodyCause, s.sourceCause, s.ownCause
}

// CleanupFormatInvocation acknowledges only the exact caller-owned invocation.
// False acknowledges completed children and the older local owning suffix.
// True aborts unjoined child roots only
// after the older scoped q/denom/num cleanup; source history remains untouched.
func (m *ArbitraryFloatManager) CleanupFormatInvocation(s *ArbitraryFloatFormatInvocation, abort bool) error {
	if !m.MatchesFormatFrame(s) {
		return umemory.ErrRuntimeOwner
	}
	if s.executing || s.finishing || m.valueExecuting || m.memory.FailureTransportActive() {
		return ErrNumericPending
	}
	if s.owned {
		return nil
	}
	s.finishing = true
	defer func() { s.finishing = false }()
	stop := func(e error) error {
		if e != nil && s.ownCause == nil {
			s.ownCause = e
		}
		return e
	}
	if e := m.cleanupFormatChildren(s, false); e != nil {
		return stop(e)
	}
	for s.ownCursor < 5 {
		var e error
		switch s.ownCursor {
		case 0:
			if s.stream != nil {
				e = s.stream.Close()
			}
		case 1:
			if s.Constructed >= 3 {
				e = m.integer.Del(&s.Q.Num)
			}
		case 2:
			if s.Constructed >= 3 {
				e = m.integer.Del(&s.Q.Den)
			}
		case 3:
			if s.Constructed >= 2 {
				e = m.integer.Del(&s.Denom)
			}
		case 4:
			if s.Constructed >= 1 {
				e = m.integer.Del(&s.Num)
			}
		}
		if e != nil {
			return stop(e)
		}
		s.ownCursor++
	}
	if !abort {
		return nil
	}
	if e := m.cleanupFormatChildren(s, true); e != nil {
		return stop(e)
	}
	s.owned = true
	s.active = false
	return nil
}

// EndFormatContext retires directory metadata only at the actual text context
// owner end, after all exact owning actions. It does not perform source DT.
func (m *ArbitraryFloatManager) EndFormatContext(s *ArbitraryFloatFormatInvocation) error {
	if !m.MatchesFormatFrame(s) {
		return umemory.ErrRuntimeOwner
	}
	if !s.owned || s.executing || s.finishing {
		return ErrNumericPending
	}
	link := &m.FormatScopes
	for *link != nil {
		if *link == s {
			*link = s.Next
			s.Next = nil
			return nil
		}
		link = &(*link).Next
	}
	return umemory.ErrRuntimeOwner
}

// The text owner uses these methods at its actual source/owning end. An unused
// context has no reached numerical invocation and therefore no NUM authority.
func (s *ArbitraryFloatFormatInvocation) SourceCause() error {
	if s == nil {
		return umemory.ErrRuntimeOwner
	}
	if s.manager == nil && s.self == nil && s.invocation == 0 {
		return nil
	}
	if s.manager == nil || !s.manager.MatchesFormatFrame(s) {
		return umemory.ErrRuntimeOwner
	}
	return s.sourceCause
}
func (s *ArbitraryFloatFormatInvocation) Cleanup(abort bool) error {
	if s == nil {
		return umemory.ErrRuntimeOwner
	}
	if s.manager == nil && s.self == nil && s.invocation == 0 {
		return nil
	}
	if s.manager == nil {
		return umemory.ErrRuntimeOwner
	}
	return s.manager.CleanupFormatInvocation(s, abort)
}
func (s *ArbitraryFloatFormatInvocation) EndContext() error {
	if s == nil {
		return umemory.ErrRuntimeOwner
	}
	if s.manager == nil && s.self == nil && s.invocation == 0 {
		return nil
	}
	if s.manager == nil {
		return umemory.ErrRuntimeOwner
	}
	return s.manager.EndFormatContext(s)
}

// SourceReady checks the literal younger callee before the text owner starts
// destroying res. It is distinct from the readonly source-history query.
func (s *ArbitraryFloatFormatInvocation) SourceReady() error {
	if s == nil {
		return umemory.ErrRuntimeOwner
	}
	if s.manager == nil && s.self == nil && s.invocation == 0 {
		return nil
	}
	if s.manager == nil || !s.manager.MatchesFormatFrame(s) {
		return umemory.ErrRuntimeOwner
	}
	if s.executing || s.finishing || s.manager.memory.FailureTransportActive() {
		return ErrNumericPending
	}
	return s.sourceCause
}
