package numeral

// Source: src/util/mpq.cpp:130-141; src/util/mpz.cpp:1385-1410.

import (
	"bytes"
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

// NumeralSourceFailure records a reached lexical source destructor stop.
// The body cause and first destructor cause remain separate through owning ACK.
type NumeralSourceFailure struct{ BodyCause, DestructorCause error }

func (e *NumeralSourceFailure) Error() string {
	return "numeric ostream source destructor stopped: " + e.DestructorCause.Error()
}
func (e *NumeralSourceFailure) Unwrap() []error { return []error{e.BodyCause, e.DestructorCause} }

// RationalSourceStream is a call-local borrowed ostream bridge, never stored in
// hot numeric records. Each method is one actual source insertion.
type RationalSourceStream interface {
	AppendInt32(int32) error
	AppendCString(string) error
	AppendNumeralText(string) error
}

// numeralCStringView borrows the exact completed lexical char buffer invocation.
// It stays inside the producer; stream callbacks receive an independent string.
type numeralCStringView struct {
	caller            *RationalServiceInvocation
	owner             *NaturalManager
	invocation        uint64
	Character         *integerCharacterScope
	displayInvocation uint64
}

func (v numeralCStringView) Byte(i uint64) (byte, error) {
	if v.owner == nil || v.owner.self != v.owner || v.owner.ClosedField {
		return 0, arena.ErrInvalidSlot
	}
	if v.caller != nil && (!v.caller.valid() || v.caller.integer.Natural != v.owner || v.Character != &v.caller.characters) {
		return 0, arena.ErrInvalidSlot
	}
	s := &v.owner.CharFormatting
	if v.Character != nil {
		s = v.Character
		if !v.owner.matchesIntegerCharacterScope(s) || v.displayInvocation != s.displayInvocation {
			return 0, arena.ErrInvalidSlot
		}
	}
	if !s.active || !s.Completed || s.invocation != v.invocation {
		return 0, arena.ErrInvalidSlot
	}
	return characterByte(s.Buffer, i)
}

// String validates the exact completed character invocation and snapshots its
// first-NUL text before crossing the output callback boundary.
func (v numeralCStringView) String() (string, error) {
	if _, e := v.Byte(0); e != nil {
		return "", e
	}
	scope := &v.owner.CharFormatting
	if v.Character != nil {
		scope = v.Character
	}
	n := bytes.IndexByte(scope.Buffer, 0)
	if n < 0 {
		return "", arena.ErrBounds
	}
	return string(scope.Buffer[:n]), nil
}

func (s *RationalService) InsertInto(out RationalSourceStream, v *OwnedRational) error {
	return s.InsertIntoTracked(out, v, nil)
}
func (s *RationalService) InsertIntoTracked(out RationalSourceStream, v *OwnedRational, receipt *NumeralInsertionReceipt) error {
	return s.insertIntoWithInvocation(out, v, receipt, nil)
}
func (s *RationalService) InsertIntoWithInvocation(out RationalSourceStream, v *OwnedRational, caller *RationalServiceInvocation) error {
	if caller == nil {
		return arena.ErrInvalidSlot
	}
	return s.insertIntoWithInvocation(out, v, &caller.insertion, caller)
}
func (s *RationalService) insertIntoWithInvocation(out RationalSourceStream, v *OwnedRational, receipt *NumeralInsertionReceipt, caller *RationalServiceInvocation) (result error) {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	s.sourceInsertionExecuting = true
	defer func() { s.sourceInsertionExecuting = false }()
	if e := s.check(v); e != nil {
		return e
	}
	if caller != nil && caller.self != nil {
		return arena.ErrInvalidSlot
	}
	defer func() {
		if caller != nil && caller.self == caller {
			caller.executing = false
			caller.bodyCause = result
			caller.sourceCause = caller.insertion.sourceCause
		}
	}()
	if out == nil {
		return arena.ErrConfig
	}
	if e := s.Manager.Z.insertSourceIntegerWithInvocation(out, &v.Raw.Num, s, receipt, caller); e != nil {
		return e
	}
	if isIntegerRational(&v.Raw) {
		return nil
	}
	if e := out.AppendCString("/"); e != nil {
		return e
	}
	return s.Manager.Z.insertSourceIntegerWithInvocation(out, &v.Raw.Den, s, receipt, caller)
}
func (m *IntegerManager) insertSourceInteger(out RationalSourceStream, a *Int) error {
	return m.insertSourceIntegerTracked(out, a, nil, nil)
}
func (m *IntegerManager) insertSourceIntegerTracked(out RationalSourceStream, a *Int, service *RationalService, receipt *NumeralInsertionReceipt) error {
	return m.insertSourceIntegerTrackedWithDisplay(out, a, service, receipt, nil)
}
func (m *IntegerManager) insertSourceIntegerTrackedWithDisplay(out RationalSourceStream, a *Int, service *RationalService, receipt *NumeralInsertionReceipt, display *NumericCharacterCaller) (result error) {
	return m.insertSourceIntegerTrackedWithContexts(out, a, service, receipt, display, nil)
}
func (m *IntegerManager) insertSourceIntegerWithInvocation(out RationalSourceStream, a *Int, service *RationalService, receipt *NumeralInsertionReceipt, caller *RationalServiceInvocation) (result error) {
	return m.insertSourceIntegerTrackedWithContexts(out, a, service, receipt, nil, caller)
}
func (m *IntegerManager) insertSourceIntegerTrackedWithContexts(out RationalSourceStream, a *Int, service *RationalService, receipt *NumeralInsertionReceipt, display *NumericCharacterCaller, caller *RationalServiceInvocation) (result error) {
	if e := m.check(a); e != nil {
		return e
	}
	if a.IsSmall() {
		return out.AppendInt32(a.Val)
	}
	if a.Val < 0 {
		if e := out.AppendCString("-"); e != nil {
			return e
		}
	}
	c := m.Cell(a.Cell)
	size, e := c.Backing.ReadWord(m.Allocator, 0)
	if e != nil {
		return e
	}
	n := m.Natural
	characters := &n.CharFormatting
	if display != nil {
		characters = display.Characters
	}
	if caller != nil {
		if caller.self == nil {
			if e = service.prepareLexicalInvocation(caller); e != nil {
				return e
			}
			service.enterLexicalInvocation()
			service.lexicalExecuting = nil
		}
		if !caller.valid() || !caller.executing || caller.integer != m || display != nil {
			return arena.ErrInvalidSlot
		}
		characters = &caller.characters
	}
	if receipt != nil && receipt.active {
		return arena.ErrInvalidSlot
	}
	previousInvocation := characters.invocation
	if caller != nil {
		e = n.beginIntegerCharactersWithCaller(uint32(11*size), service, receipt, characters, caller)
	} else {
		e = n.beginIntegerCharactersForInsertion(uint32(11*size), service, receipt, display)
	}
	if characters.invocation != previousInvocation && characters.callExecuting {
		defer n.endIntegerCharacterCall(characters)
	}
	if e != nil {
		if receipt != nil && receipt.active {
			receipt.bodyCause = e
			receipt.sourceDone = true
		}
		return e
	}
	defer func() {
		if receipt != nil && receipt.active {
			if result != nil && receipt.bodyCause == nil {
				receipt.bodyCause = result
			}
			var stopped *NumeralSourceFailure
			if errors.As(result, &stopped) {
				receipt.sourceCause = stopped.DestructorCause
				return
			}
			receipt.sourceDone = true
			if result == nil {
				receipt.active = false
				receipt.retired = true
			}
		}
	}()
	e = n.sourceToCStringScopedTracked(limbView{allocator: m.Allocator, backing: c.Backing, size: size}, sourceDecimalDestination{character: characters.Buffer}, uint64(len(characters.Buffer)), characters.formatting, receipt)
	if e == nil {
		view := numeralCStringView{owner: n, invocation: characters.invocation, Character: characters, displayInvocation: characters.displayInvocation, caller: caller}
		var text string
		text, e = view.String()
		if e == nil {
			e = out.AppendNumeralText(text)
		}
	}
	if e != nil && characters.originalCause == nil {
		characters.originalCause = e
	}
	// A younger source local that stopped destruction protects the older char
	// buffer. Failed construction likewise has no source destructor.
	var stopped *NumeralSourceFailure
	if errors.As(e, &stopped) {
		return e
	}
	if cleanup := n.destroyIntegerCharacterScope(characters); cleanup != nil {
		return &NumeralSourceFailure{BodyCause: e, DestructorCause: cleanup}
	}
	if receipt != nil {
		receipt.characterOwned = false
	}
	return e
}

// InsertSourceIntegerTracked borrows caller-owned character scopes for this source insertion.
func (m *IntegerManager) InsertSourceIntegerTracked(out RationalSourceStream, a *Int, receipt *NumeralInsertionReceipt, caller *NumericCharacterCaller) error {
	return m.insertSourceIntegerTrackedWithDisplay(out, a, nil, receipt, caller)
}
