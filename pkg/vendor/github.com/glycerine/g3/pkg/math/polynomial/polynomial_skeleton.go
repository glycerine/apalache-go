package polynomial

// Source: src/math/polynomial/polynomial.cpp:2958-3068.

import (
	"reflect"
	"sort"

	"github.com/glycerine/g3/pkg/arena"
)

type PolynomialSkeletonEntry struct {
	Monomial              Monomial
	FirstPower, NumPowers uint32
}

// Entries and originals own their actual monomial references. A failed
// constructor retains those references for CloseChecked without running the
// source class destructor; native storage needs no separate unwind transport.
type PolynomialSkeleton struct {
	self                                                       *PolynomialSkeleton
	owner                                                      *MultivariatePolynomialCore
	x, maxPowers                                               uint32
	entries                                                    []PolynomialSkeletonEntry
	powers                                                     []uint32
	originals                                                  []Monomial
	header                                                     uint64
	originalCause, sourceCause                                 error
	completed, closing, closed, sourceStarted, sourceFinalized bool
	entryCursor, originalCursor                                uint32
	entryDropping, originalDropping                            bool
	generated                                                  Monomial
	generatedActive, calleeCreation                            bool
	memoryTraceLive                                            bool
}

func NewPolynomialSkeleton(m *MultivariatePolynomialCore, p Polynomial, x uint32) (*PolynomialSkeleton, error) {
	return newPolynomialSkeleton(m, p, x, false)
}

// Only the persistent sparse-GCD child is allocated by the source
// polynomial.cpp:4226 alloc site. Other Go skeleton construction is local.
func newGCDPolynomialSkeleton(m *MultivariatePolynomialCore, p Polynomial, x uint32) (*PolynomialSkeleton, error) {
	return newPolynomialSkeleton(m, p, x, true)
}

func newPolynomialSkeleton(m *MultivariatePolynomialCore, p Polynomial, x uint32, gcdAllocation bool) (*PolynomialSkeleton, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	if _, e := m.polynomialLocation(p); e != nil {
		return nil, e
	}
	h := uint64(reflect.TypeOf(PolynomialSkeleton{}).Size())
	if e := m.integer.Budget().Reserve(h); e != nil {
		return nil, e
	}
	s := &PolynomialSkeleton{owner: m, x: x, header: h}
	s.self = s
	if gcdAllocation {
		// C++ alloc emits its memory record after storage allocation and
		// before the placement constructor evaluates its first member.
		s.traceGCDSkeletonAllocated()
	}
	fail := func(e error) (*PolynomialSkeleton, error) {
		s.originalCause = e
		return s, s
	}
	n, e := m.readWord(p, 8)
	if e != nil {
		return fail(e)
	}
	ms := make([]Monomial, 0, int(n))
	for i := uint32(0); i < n; i++ {
		a, e := m.monomialCore(p, i)
		if e != nil {
			return fail(e)
		}
		ms = append(ms, a)
	}
	// Only borrowed local handles are sorted. Once a comparison fails, do not
	// read another monomial; return the actual owner error before any INC.
	var sortErr error
	sort.Slice(ms, func(i, j int) bool {
		if sortErr != nil {
			return false
		}
		if e := m.monomials.traceSkeletonLexCompare(ms[i], ms[j], x); e != nil {
			sortErr = e
			return false
		}
		v, e := m.monomials.LexWithMinimum(ms[i], ms[j], x)
		sortErr = e
		return e == nil && v < 0
	})
	if sortErr != nil {
		return fail(sortErr)
	}
	var previous Monomial
	for _, orig := range ms {
		k, e := m.monomials.DegreeOf(orig, x)
		if e != nil {
			return fail(e)
		}
		a := orig
		if k > 0 {
			var outcome monomialFactoryOutcome
			a, e = m.monomials.removeVariableCreated(orig, x, &outcome)
			s.calleeCreation = outcome.started
			if e != nil {
				return fail(e)
			}
			s.calleeCreation = false
			if outcome.created {
				s.generated = a
				s.generatedActive = true
			}
		}
		if a == previous {
			if len(s.entries) == 0 {
				return fail(arena.ErrBounds)
			}
			entry := &s.entries[len(s.entries)-1]
			entry.NumPowers++
			if entry.NumPowers > s.maxPowers {
				s.maxPowers = entry.NumPowers
			}
		} else {
			previous = a
			if e = m.monomials.IncRef(a); e != nil {
				return fail(e)
			}
			s.entries = append(s.entries, PolynomialSkeletonEntry{a, uint32(len(s.powers)), 1})
			s.generated = Monomial{}
			s.generatedActive = false
			if s.maxPowers == 0 {
				s.maxPowers = 1
			}
		}
		if e = m.monomials.IncRef(orig); e != nil {
			return fail(e)
		}
		s.originals = append(s.originals, orig)
		s.powers = append(s.powers, k)
	}
	if e := s.traceConstruction(p); e != nil {
		return fail(e)
	}
	if polynomialSourceDebug {
		for i := 1; i < len(s.entries); i++ {
			left, right := s.entries[i-1].Monomial, s.entries[i].Monomial
			if e := polynomialSkeletonSourceAssert("src/math/polynomial/polynomial.cpp", 3020, "lex_compare(m_entries[i-1].m_monomial, m_entries[i].m_monomial) < 0", func() (bool, error) {
				v, x := m.monomials.Lex(left, right)
				return v < 0, x
			}); e != nil {
				return fail(e)
			}
		}
	}
	s.completed = true
	return s, nil
}
func (s *PolynomialSkeleton) Error() string {
	if s.originalCause != nil {
		return s.originalCause.Error()
	}
	return ErrPolynomialPending.Error()
}
func (s *PolynomialSkeleton) Unwrap() error        { return s.originalCause }
func (s *PolynomialSkeleton) OriginalCause() error { return s.originalCause }
func (s *PolynomialSkeleton) CleanupCause() error  { return nil }
func (s *PolynomialSkeleton) ready() error {
	if s == nil || s.self != s {
		return ErrPolynomialOwner
	}
	if s.closed || s.closing || s.sourceStarted || s.originalCause != nil {
		return arena.ErrClosed
	}
	return s.owner.check()
}
func skeletonNativeIndex(i uint32, n int) error {
	if uint64(i) < uint64(n) {
		return nil
	}
	return arena.ErrBounds
}
func skeletonIndex(i uint32, n int) error { return polynomialSkeletonVectorIndex(i, n) }
func (s *PolynomialSkeleton) entryMonomialCore(i uint32) (Monomial, error) {
	if e := skeletonIndex(i, len(s.entries)); e != nil {
		return Monomial{}, e
	}
	return s.entries[i].Monomial, nil
}
func (s *PolynomialSkeleton) NumEntries() (uint32, error) {
	if e := s.ready(); e != nil {
		return 0, e
	}
	return uint32(len(s.entries)), nil
}
func (s *PolynomialSkeleton) MaxNumPowers() (uint32, error) {
	if e := s.ready(); e != nil {
		return 0, e
	}
	return s.maxPowers, nil
}
func (s *PolynomialSkeleton) Entry(i uint32) (PolynomialSkeletonEntry, error) {
	if e := s.ready(); e != nil {
		return PolynomialSkeletonEntry{}, e
	}
	if e := skeletonIndex(i, len(s.entries)); e != nil {
		return PolynomialSkeletonEntry{}, e
	}
	return s.entries[i], nil
}
func (s *PolynomialSkeleton) EntryIndex(a Monomial) (uint32, error) {
	if e := s.ready(); e != nil {
		return 0, e
	}
	for i, entry := range s.entries {
		if a == entry.Monomial {
			return uint32(i), nil
		}
	}
	return ^uint32(0), nil
}
func (s *PolynomialSkeleton) Power(a PolynomialSkeletonEntry, i uint32) (uint32, error) {
	if e := s.ready(); e != nil {
		return 0, e
	}
	if e := polynomialSkeletonScalarAssert("src/math/polynomial/polynomial.cpp", 3049, "i < e.m_num_powers", i < a.NumPowers); e != nil {
		return 0, e
	}
	index := a.FirstPower + i
	if e := skeletonIndex(index, len(s.powers)); e != nil {
		return 0, e
	}
	return s.powers[index], nil
}
func (s *PolynomialSkeleton) Original(a PolynomialSkeletonEntry, i uint32) (Monomial, error) {
	if e := s.ready(); e != nil {
		return Monomial{}, e
	}
	if e := polynomialSkeletonScalarAssert("src/math/polynomial/polynomial.cpp", 3051, "i < e.m_num_powers", i < a.NumPowers); e != nil {
		return Monomial{}, e
	}
	index := a.FirstPower + i
	if e := skeletonIndex(index, len(s.originals)); e != nil {
		return Monomial{}, e
	}
	return s.originals[index], nil
}
func skeletonDrop(s *PolynomialSkeleton, a *Monomial, started *bool, owning bool) error {
	if *a == (Monomial{}) {
		return nil
	}
	if *started {
		if !owning {
			return s.sourceCause
		}
		return s.owner.monomials.CleanupValue(a)
	}
	e := s.owner.monomials.decRefCore(*a)
	if e != nil {
		l, x := s.owner.monomials.location(*a)
		if x == nil && l.stage != 0 {
			*started = true
		}
		return e
	}
	*a = Monomial{}
	return nil
}
func (s *PolynomialSkeleton) destroyEntries(owning bool) error {
	for uint64(s.entryCursor) < uint64(len(s.entries)) {
		a := s.entries[s.entryCursor].Monomial
		if e := skeletonDrop(s, &a, &s.entryDropping, owning); e != nil {
			return e
		}
		s.entryDropping = false
		s.entryCursor++
	}
	return nil
}
func (s *PolynomialSkeleton) destroyOriginals(owning bool) error {
	for uint64(s.originalCursor) < uint64(len(s.originals)) {
		a := s.originals[s.originalCursor]
		if e := skeletonDrop(s, &a, &s.originalDropping, owning); e != nil {
			return e
		}
		s.originalDropping = false
		s.originalCursor++
	}
	return nil
}
func (s *PolynomialSkeleton) SourceFinalize() error {
	if s == nil || s.self != s {
		return ErrPolynomialOwner
	}
	if s.owner.sparseExecuting(s) {
		return ErrPolynomialPending
	}
	if s.closed || s.closing || !s.completed {
		return arena.ErrClosed
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if s.sourceFinalized {
		return nil
	}
	s.sourceStarted = true
	if e := s.destroyEntries(false); e != nil {
		s.sourceCause = e
		return e
	}
	if e := s.destroyOriginals(false); e != nil {
		s.sourceCause = e
		return e
	}
	s.originals = nil
	s.powers = nil
	s.entries = nil
	s.sourceFinalized = true
	return nil
}
func (s *PolynomialSkeleton) CloseChecked() error {
	if s == nil || s.self != s {
		return ErrPolynomialOwner
	}
	if s.owner.sparseExecuting(s) {
		return ErrPolynomialPending
	}
	if s.closed {
		return nil
	}
	s.closing = true
	if e := s.destroyEntries(true); e != nil {
		return e
	}
	if e := s.destroyOriginals(true); e != nil {
		return e
	}
	if s.generatedActive {
		if e := s.owner.monomials.CleanupValue(&s.generated); e != nil {
			return e
		}
		s.generatedActive = false
	}
	if s.calleeCreation {
		if e := s.owner.monomials.CleanupCreation(); e != nil {
			return e
		}
		s.calleeCreation = false
	}
	s.originals = nil
	s.powers = nil
	s.entries = nil
	s.owner.integer.Budget().Release(s.header)
	s.header = 0
	s.closed = true
	return nil
}

// Display re-reads entry fields after output callbacks, as the source entry
// reference does. No snapshot of the full entry survives a callback.
func (s *PolynomialSkeleton) DisplayInto(out MonomialTextSink) error {
	if e := s.ready(); e != nil {
		return e
	}
	n := uint32(len(s.entries))
	entry := func(i uint32) (PolynomialSkeletonEntry, error) {
		if e := s.ready(); e != nil {
			return PolynomialSkeletonEntry{}, e
		}
		if e := skeletonIndex(i, len(s.entries)); e != nil {
			return PolynomialSkeletonEntry{}, e
		}
		return s.entries[i], nil
	}
	for i := uint32(0); i < n; i++ {
		if e := skeletonIndex(i, len(s.entries)); e != nil {
			return e
		}
		if i > 0 {
			if e := monomialSinkLiteral(out, " "); e != nil {
				return e
			}
		}
		if e := monomialSinkLiteral(out, "("); e != nil {
			return e
		}
		for j := uint32(0); ; j++ {
			a, e := entry(i)
			if e != nil {
				return e
			}
			if j >= a.NumPowers {
				break
			}
			if j > 0 {
				if e := monomialSinkLiteral(out, " "); e != nil {
					return e
				}
			}
			if e := monomialSinkLiteral(out, "x"); e != nil {
				return e
			}
			if e := s.ready(); e != nil {
				return e
			}
			if e := monomialSinkUnsigned(out, s.x); e != nil {
				return e
			}
			if e := monomialSinkLiteral(out, "^"); e != nil {
				return e
			}
			a, e = entry(i)
			if e != nil {
				return e
			}
			index := a.FirstPower + j
			if e := skeletonIndex(index, len(s.powers)); e != nil {
				return e
			}
			if e := monomialSinkUnsigned(out, s.powers[index]); e != nil {
				return e
			}
		}
		if e := monomialSinkLiteral(out, ")*"); e != nil {
			return e
		}
		if e := s.ready(); e != nil {
			return e
		}
		a, e := s.entryMonomialCore(i)
		if e != nil {
			return e
		}
		if e := s.owner.monomials.DisplayInto(a, out, false); e != nil {
			return e
		}
	}
	return nil
}
