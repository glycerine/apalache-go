package numeral

// Source: src/util/mpq.cpp; src/util/scoped_numeral.h.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

var ErrRationalLexicalPending = errors.New("mpnum: owning lexical cleanup is pending")

// RationalLexicalState describes actual embedded owning records, not allocator
// registry membership. Completed roots precede partial-constructor abort roots.
type RationalLexicalState struct {
	Roots, Completed, Aborts uint8
	Pending                  bool
	Cause, OriginalCause     error
}

// Nine includes the external ext_gcd source stack: six persistent locals,
// coefficient product, completed inner expression work, and partial returned-
// Copy adapter token. Destructor refusal can retain all nine records. Existing
// Inf expression stacks fit eight. All fields are charged by actual owner size.
const rationalLexicalRootLimit = 9

type rationalLexicalScope struct {
	Roots                [rationalLexicalRootLimit]Rational
	abort                [rationalLexicalRootLimit]bool
	components           [rationalLexicalRootLimit]uint8
	Count, cursor, depth uint8
	Done                 uint16
	Cause, originalCause error
}

func (l *rationalLexicalScope) Pending() bool { return l.cursor < l.Count }
func (l *rationalLexicalScope) retain(a *Rational, abort bool) uint8 {
	if a.Num.Cell == 0 && a.Den.Cell == 0 {
		*a = Rational{}
		return rationalLexicalRootLimit
	}
	if l.Count == rationalLexicalRootLimit {
		panic("mpnum: audited lexical root bound violated")
	}
	index := l.Count
	l.Roots[index] = *a
	l.abort[index] = abort
	l.Count++
	*a = Rational{}
	return index
}
func (l *rationalLexicalScope) State() RationalLexicalState {
	v := RationalLexicalState{Pending: l.Pending(), Cause: l.Cause, OriginalCause: l.originalCause}
	for i := uint8(0); i < l.Count; i++ {
		if l.Done&(1<<i) != 0 {
			continue
		}
		v.Roots++
		if l.abort[i] {
			v.Aborts++
		} else {
			v.Completed++
		}
	}
	return v
}
func lexicalDelete(z *IntegerManager, a *Rational, abort bool, component *uint8) error {
	for *component < 2 {
		p := &a.Num
		if *component == 1 {
			p = &a.Den
		}
		if abort { // explicitly requested Go owning abort; source pointer state is not guessed
			if e := z.Del(p); e != nil {
				return e
			}
		} else {
			if _, e := sourceNumeralDel(z, p); e != nil {
				return e
			}
		}
		*component++
	}
	*a = Rational{}
	return nil
}

// Failed-constructor storage is an owning Go obligation. It is queued until
// every completed source object in the enclosing lexical stack has unwound.
func (l *rationalLexicalScope) dispose(z *IntegerManager, a *Rational, abort bool) error {
	if abort {
		l.retain(a, true)
		return l.Cause
	}
	if l.Cause != nil {
		l.retain(a, false)
		return l.Cause
	}
	var component uint8
	if e := lexicalDelete(z, a, false, &component); e != nil {
		l.Cause = e
		index := l.retain(a, false)
		if index < rationalLexicalRootLimit {
			l.components[index] = component
		}
		return e
	}
	return nil
}
func (l *rationalLexicalScope) cleanup(z *IntegerManager) error {
	// Completed source destructor order precedes separate partial-owning aborts,
	// even when a partial inner copy was queued before the outer unwind began.
	for l.Pending() {
		index := uint8(rationalLexicalRootLimit)
		for i := uint8(0); i < l.Count; i++ {
			if l.Done&(1<<i) == 0 && !l.abort[i] {
				index = i
				break
			}
		}
		if index == rationalLexicalRootLimit {
			for i := uint8(0); i < l.Count; i++ {
				if l.Done&(1<<i) == 0 {
					index = i
					break
				}
			}
		}
		if e := lexicalDelete(z, &l.Roots[index], true, &l.components[index]); e != nil {
			l.Cause = e
			return e
		}
		l.Done |= 1 << index
		for l.cursor < l.Count && l.Done&(1<<l.cursor) != 0 {
			l.cursor++
		}
	}
	depth := l.depth
	*l = rationalLexicalScope{depth: depth}
	return nil
}
func (s *RationalService) LexicalState() (RationalLexicalState, error) {
	if s == nil || s.self != s {
		return RationalLexicalState{}, ErrRationalServiceCopy
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Lexical.State(), nil
}
func (s *RationalService) CleanupLexical() error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if s.Lexical.depth != 0 {
		return ErrRationalLexicalPending
	}
	return s.Lexical.cleanup(s.Manager.Z)
}
func (s *RationalService) lexicalReady() error {
	if s == nil || s.self != s {
		return ErrRationalServiceCopy
	}
	if s.Lexical.Pending() {
		return ErrRationalLexicalPending
	}
	return nil
}
func (s *RationalService) destroyLexicalRational(a *Rational) error {
	return s.Lexical.dispose(s.Manager.Z, a, false)
}
func (s *RationalService) abortLexicalRational(a *Rational) error {
	return s.Lexical.dispose(s.Manager.Z, a, true)
}

// Local completion flags are concrete stack scalars; no scope ticket or heap
// context is created for an expression, and no callback/function variable is stored.
func (s *RationalService) beginLexical() error {
	if e := s.lexicalReady(); e != nil {
		return e
	}
	if e := s.Manager.checkOwnerReady(); e != nil {
		return e
	}
	if s.Lexical.depth == 255 {
		return arena.ErrBounds
	}
	if s.Lexical.depth == 0 {
		if s.lexicalGeneration == math.MaxUint64 {
			return arena.ErrBounds
		}
		s.lexicalGeneration++
	}
	s.enterLexicalInvocation()
	s.Lexical.depth++
	return nil
}
func (s *RationalService) endLexicalLocal(a *Rational, completed *bool) {
	if *completed {
		s.destroyLexicalRational(a)
	} else {
		s.abortLexicalRational(a)
	}
}
func (s *RationalService) endLexicalError(e *error) {
	s.finishLexicalError(e)
	if s.Lexical.depth > 0 {
		s.Lexical.depth--
	}
	if s.Lexical.depth == 0 && s.Lexical.Cause == nil && s.lexicalExecuting == nil {
		if next := s.Lexical.cleanup(s.Manager.Z); next != nil && *e == nil {
			*e = next
		}
	}
	s.finishLexicalError(e)
}
func (s *RationalService) endLexicalRational(out *OwnedRational, e *error) {
	s.endLexicalError(e)
	s.finishLexicalRational(out, e)
}
func (s *RationalService) endLexicalInf(out *OwnedInfRational, e *error) {
	s.endLexicalError(e)
	s.finishLexicalInf(out, e)
}
func (s *RationalService) endLexicalInfInt(out *OwnedInfIntRational, e *error) {
	s.endLexicalError(e)
	s.finishLexicalInfInt(out, e)
}
func (s *RationalService) endLexicalEps(out *OwnedInfEpsInfRational, e *error) {
	s.endLexicalError(e)
	s.finishLexicalEps(out, e)
}
func (s *RationalService) finishLexicalError(e *error) {
	if s.Lexical.Pending() && *e != nil && s.Lexical.originalCause == nil && !errors.Is(*e, s.Lexical.Cause) {
		s.Lexical.originalCause = *e
	}
	if s.Lexical.Pending() && *e == nil {
		*e = s.Lexical.Cause
	}
}
func (s *RationalService) finishLexicalRational(out *OwnedRational, e *error) {
	if s.Lexical.Pending() {
		if out.ownerID != 0 {
			s.destroyLexicalRational(&out.Raw)
			*out = OwnedRational{}
		}
		s.finishLexicalError(e)
	}
}
func (s *RationalService) finishLexicalInf(out *OwnedInfRational, e *error) {
	if s.Lexical.Pending() {
		if out.ownerID != 0 {
			s.infDestroy(out)
		}
		s.finishLexicalError(e)
	}
}
func (s *RationalService) finishLexicalInfInt(out *OwnedInfIntRational, e *error) {
	if s.Lexical.Pending() {
		if out.ownerID != 0 {
			s.destroyLexicalRational(&out.First)
			*out = OwnedInfIntRational{}
		}
		s.finishLexicalError(e)
	}
}
func (s *RationalService) finishLexicalEps(out *OwnedInfEpsInfRational, e *error) {
	if s.Lexical.Pending() {
		if out.ownerID != 0 {
			s.infEpsInfDestroy(out)
		}
		s.finishLexicalError(e)
	}
}
func (m *RationalManager) CleanupLexical() error {
	if e := m.checkOwnerReady(); e != nil {
		return e
	}
	return m.lexical.cleanup(m.Z)
}
func (m *RationalManager) LexicalState() (RationalLexicalState, error) {
	if m == nil || m.self != m {
		return RationalLexicalState{}, arena.ErrInvalidSlot
	}
	return m.lexical.state(), nil
}

// Cross-owner Rational.CopyFrom has exactly one staged Rational local.
type rationalManagerLexicalScope struct {
	roots                    [1]Rational
	abort                    [1]bool
	count, cursor, component uint8
	cause, originalCause     error
}

func (l *rationalManagerLexicalScope) pending() bool { return l.cursor < l.count }
func (l *rationalManagerLexicalScope) retain(a *Rational, abort bool) {
	if a.Num.Cell == 0 && a.Den.Cell == 0 {
		*a = Rational{}
		return
	}
	// Every caller is a fixed source expression graph bounded above. Never drop
	// ownership on a violated bound: callers stop before producing another local.
	if l.count == uint8(len(l.roots)) {
		panic("mpnum: audited lexical root bound violated")
	}
	l.roots[l.count] = *a
	l.abort[l.count] = abort
	l.count++
	*a = Rational{}
}
func (l *rationalManagerLexicalScope) state() RationalLexicalState {
	v := RationalLexicalState{Pending: l.pending(), Cause: l.cause, OriginalCause: l.originalCause}
	for i := l.cursor; i < l.count; i++ {
		v.Roots++
		if l.abort[i] {
			v.Aborts++
		} else {
			v.Completed++
		}
	}
	return v
}
func (l *rationalManagerLexicalScope) dispose(z *IntegerManager, a *Rational, abort bool) error {
	if l.pending() {
		l.retain(a, abort)
		return l.cause
	}
	var component uint8
	if e := lexicalDelete(z, a, abort, &component); e != nil {
		l.cause = e
		l.retain(a, abort)
		l.component = component
		return e
	}
	return nil
}
func (l *rationalManagerLexicalScope) cleanup(z *IntegerManager) error {
	for l.pending() {
		// This public recovery is owning Go cleanup; it may abort a source-invalid
		// failed-copy pointer. Integer.Del resumes its persisted physical release.
		if e := lexicalDelete(z, &l.roots[l.cursor], true, &l.component); e != nil {
			l.cause = e
			return e
		}
		l.cursor++
		l.component = 0
	}
	*l = rationalManagerLexicalScope{}
	return nil
}

func (m *RationalManager) finishLexicalCopy(a *Rational, e *error) {
	if next := m.lexical.dispose(m.Z, a, false); next != nil {
		if *e == nil {
			*e = next
		} else {
			m.lexical.originalCause = *e
		}
	}
}

// Public Del retains the caller's scalar token on refusal. Each callback is
// reached in source order; no unaccessed later component is prevalidated.
func callerDeleteRational(z *IntegerManager, a *Rational) error {
	if e := z.Del(&a.Num); e != nil {
		return e
	}
	return z.Del(&a.Den)
}
