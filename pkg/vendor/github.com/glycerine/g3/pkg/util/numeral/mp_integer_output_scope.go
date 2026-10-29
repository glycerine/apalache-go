package numeral

// Source: src/util/mpz.cpp:1385-1410; src/util/buffer.h:84-95.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

type integerCharacterScope struct {
	caller                      *RationalServiceInvocation
	self                        *integerCharacterScope
	owner                       *NaturalManager
	display                     *NumericCharacterCaller
	displayInvocation           uint64
	formatting                  *naturalFormattingScope
	previous                    *integerCharacterScope
	callExecuting               bool
	Buffer                      []byte
	invocation                  uint64
	active, Completed, pending  bool
	originalCause, CleanupCause error
}

func (m *NaturalManager) beginIntegerCharacters(n uint32) error {
	return m.beginIntegerCharactersTracked(n, nil, nil)
}
func (m *NaturalManager) beginIntegerCharactersTracked(n uint32, service *RationalService, receipt *NumeralInsertionReceipt) error {
	return m.beginIntegerCharactersWithDisplay(n, service, receipt, nil)
}
func (m *NaturalManager) beginIntegerCharactersWithDisplay(n uint32, service *RationalService, receipt *NumeralInsertionReceipt, display *NumericCharacterCaller) error {
	return m.beginIntegerCharacterScope(n, service, receipt, display, false)
}
func (m *NaturalManager) beginIntegerCharactersForInsertion(n uint32, service *RationalService, receipt *NumeralInsertionReceipt, display *NumericCharacterCaller) error {
	return m.beginIntegerCharacterScope(n, service, receipt, display, true)
}
func (m *NaturalManager) beginIntegerCharacterScope(n uint32, service *RationalService, receipt *NumeralInsertionReceipt, display *NumericCharacterCaller, executing bool) error {
	return m.beginIntegerCharacterScopeWithCaller(n, service, receipt, display, executing, nil)
}
func (m *NaturalManager) beginIntegerCharactersWithCaller(n uint32, service *RationalService, receipt *NumeralInsertionReceipt, scope *integerCharacterScope, caller *RationalServiceInvocation) error {
	if caller == nil || scope != &caller.characters {
		return arena.ErrInvalidSlot
	}
	return m.beginIntegerCharacterScopeWithCaller(n, service, receipt, nil, true, caller)
}
func (m *NaturalManager) beginIntegerCharacterScopeWithCaller(n uint32, service *RationalService, receipt *NumeralInsertionReceipt, display *NumericCharacterCaller, executing bool, caller *RationalServiceInvocation) error {
	if e := m.check(); e != nil {
		return e
	}
	s := &m.CharFormatting
	formatting := &m.Formatting
	if display != nil {
		if !display.valid(m, true) || display.Characters == nil || display.Formatting == nil {
			return ErrNumericCallerOwner
		}
		s = display.Characters
		formatting = display.Formatting
	}
	if caller != nil {
		if !caller.valid() || !caller.executing || caller.integer.Natural != m || display != nil {
			return arena.ErrInvalidSlot
		}
		s = &caller.characters
		formatting = &caller.characterFormatting
	}
	if uint64(n) > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	if s.active || s.pending {
		return arena.ErrInvalidSlot
	}
	if s.invocation == math.MaxUint64 {
		return arena.ErrBounds
	}
	if display != nil {
		if e := display.prepare(); e != nil {
			return e
		}
	}
	buffer := s.Buffer
	*s = integerCharacterScope{Buffer: buffer, caller: caller, owner: m, display: display, formatting: formatting, active: true, invocation: s.invocation + 1}
	s.self = s
	if display != nil {
		s.displayInvocation = display.Generation
	}
	if receipt != nil {
		*receipt = NumeralInsertionReceipt{service: service, natural: m, Character: s, invocation: s.invocation, displayInvocation: s.displayInvocation, active: true, characterOwned: true}
	}
	if executing {
		m.beginIntegerCharacterCall(s)
	}
	if int(n) > cap(s.Buffer) {
		s.Buffer = make([]byte, n)
	} else {
		s.Buffer = s.Buffer[:n]
		clear(s.Buffer)
	}

	s.Completed = true
	if receipt != nil {
		receipt.characterCompleted = true
	}
	return nil
}
func (m *NaturalManager) destroyIntegerCharacters() error {
	return m.destroyIntegerCharacterScope(&m.CharFormatting)
}
func (m *NaturalManager) destroyIntegerCharacterScope(s *integerCharacterScope) error {
	if !s.Completed {
		return nil
	}
	s.Completed = false
	s.active = false

	s.reset()
	return nil
}

func (s *integerCharacterScope) reset() {
	*s = integerCharacterScope{Buffer: s.Buffer[:0], caller: s.caller, self: s.self, owner: s.owner, display: s.display, displayInvocation: s.displayInvocation, formatting: s.formatting, previous: s.previous, callExecuting: s.callExecuting, invocation: s.invocation}
}

func (m *NaturalManager) matchesIntegerCharacterScope(s *integerCharacterScope) bool {
	if m == nil || m.self != m || s == nil || s.self != s || s.owner != m {
		return false
	}
	if s == &m.CharFormatting {
		return s.display == nil && s.formatting == &m.Formatting
	}
	if c := s.caller; c != nil {
		return c.valid() && c.integer.Natural == m && s == &c.characters && s.formatting == &c.characterFormatting
	}
	q := s.display
	return q != nil && q.valid(m, false) && s == q.Characters && s.displayInvocation != 0 && s.displayInvocation == q.Generation && s.formatting == q.Formatting
}

func (m *NaturalManager) beginIntegerCharacterCall(s *integerCharacterScope) {
	s.previous = m.characterCalls
	s.callExecuting = true
	m.characterCalls = s
}
func (m *NaturalManager) endIntegerCharacterCall(s *integerCharacterScope) {
	if m.characterCalls == s && s.callExecuting {
		m.characterCalls = s.previous
		s.previous = nil
		s.callExecuting = false
	}
}

// cleanupOutputFormatting ends numerical locals before invalidating borrowed
// character views, then reclaims retained numerical allocations and Go scratch.
func (m *NaturalManager) cleanupOutputFormatting() error {
	if m.Formatting.Active || m.Formatting.CompletedField != 0 {
		if e := m.unwindFormattingLocals(); e != nil {
			return e
		}
		m.Formatting.Active = false
		m.Formatting.PendingField = true
	}
	if e := m.destroyIntegerCharacters(); e != nil {
		return e
	}
	if e := m.CleanupFormatting(); e != nil {
		return e
	}
	if m.CharFormatting.pending {
		m.CharFormatting = integerCharacterScope{invocation: m.CharFormatting.invocation}
	}
	m.CharFormatting.Buffer = nil
	return nil
}

func (m *NaturalManager) appendIntegerCString(offset uint32) (uint32, error) {
	s := &m.CharFormatting
	var n uint64
	for {
		v, e := characterByte(s.Buffer, n)
		if e != nil {
			return 0, e
		}
		if v == 0 {
			break
		}
		n++
	}
	end, e := checkedTextEnd(offset, n)
	if e != nil {
		return 0, e
	}
	// This retained byte assembly is an explicit Go output adapter, not the
	// source character buffer or a fictitious fresh source allocation.
	if e = m.reserveText(end); e != nil {
		return 0, e
	}
	for i := uint64(0); i < n; i++ {
		v, e := characterByte(s.Buffer, i)
		if e != nil {
			return 0, e
		}
		m.textWrite(offset+uint32(i), v)
	}
	return end, nil
}
