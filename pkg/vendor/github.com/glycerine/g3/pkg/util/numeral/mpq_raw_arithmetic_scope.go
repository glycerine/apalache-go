package numeral

// Source: src/util/mpq.h; src/util/mpq.cpp.

import (
	"errors"
)

var ErrRationalRawArithmeticPending = errors.New("mpnum: raw arithmetic local cleanup pending")

// Actual cold owner-local storage for source RAW locals: Power's rational,
// SYNCH add's two mpz, normalize's mpz, divisor-alias division's mpz, and
// SYNCH comparison's two mpq.
// The ten stored Int fields are charged once in the manager header. Comparison
// denominators are normally inline1; stopped independent source calls may retain
// separate cohorts. No source exception implicitly destroys these raw objects.
type rationalRawArithmeticScope struct {
	Div                                            Int
	DivActive, divEnd, divGuard                    bool
	divCursor                                      uint8
	divCause                                       error
	Power                                          Rational
	CompareOne, CompareTwo                         Rational
	CompareActive, compareEnd, compareGuard        bool
	compareCursor                                  uint8
	compareCause                                   error
	AddOne, AddTwo, Norm                           Int
	PowerActive, AddActive, NormActive             bool
	powerEnd, addEnd, normEnd                      bool
	powerGuard, addGuard, normGuard                bool
	powerCursor, addCursor                         uint8
	powerCause, addCause, normCause                error
	PowerGeneration, addGeneration, normGeneration uint64
	compareGeneration, divGeneration               uint64
}

type RationalRawArithmeticState struct {
	Division, DivisionEnd, DivisionGuard          bool
	DivisionCursor                                uint8
	DivisionCause                                 error
	Power, Addition, Normalization                bool
	Comparison, ComparisonEnd, ComparisonGuard    bool
	ComparisonCursor                              uint8
	ComparisonCause                               error
	PowerEnd, AdditionEnd, NormalizationEnd       bool
	PowerGuard, AdditionGuard, NormalizationGuard bool
	PowerCursor, AdditionCursor                   uint8
	PowerCause, AdditionCause, NormalizationCause error
}

func (r *rationalRawArithmeticScope) Pending() bool {
	return r.PowerActive || r.AddActive || r.NormActive || r.CompareActive || r.DivActive
}
func (m *RationalManager) RawArithmeticState() (RationalRawArithmeticState, error) {
	if e := m.checkOwnerReady(); e != nil {
		return RationalRawArithmeticState{}, e
	}
	r := &m.RawArithmetic
	return RationalRawArithmeticState{Division: r.DivActive, DivisionEnd: r.divEnd, DivisionGuard: r.divGuard, DivisionCursor: r.divCursor, DivisionCause: r.divCause, Comparison: r.CompareActive, ComparisonEnd: r.compareEnd, ComparisonGuard: r.compareGuard, ComparisonCursor: r.compareCursor, ComparisonCause: r.compareCause, Power: r.PowerActive, Addition: r.AddActive, Normalization: r.NormActive, PowerEnd: r.powerEnd, AdditionEnd: r.addEnd, NormalizationEnd: r.normEnd, PowerGuard: r.powerGuard, AdditionGuard: r.addGuard, NormalizationGuard: r.normGuard, PowerCursor: r.powerCursor, AdditionCursor: r.addCursor, PowerCause: r.powerCause, AdditionCause: r.addCause, NormalizationCause: r.normCause}, nil
}

// Div's raw local has no C++ exception destructor. The explicit source
// end-label occurs after NUM Copy and before denominator sign repair.
func (m *RationalManager) finishRawDiv(e *error) {
	r := &m.RawArithmetic
	if *e != nil && r.DivActive {
		r.divCause = *e
		if r.Div.Cell == 0 {
			r.DivActive = false
			r.Div = Int{}
		}
	}
}
func (m *RationalManager) rawDivEnd() error {
	r := &m.RawArithmetic
	r.divEnd = true
	guard, e := sourceNumeralDel(m.Z, &r.Div)
	r.divGuard = guard
	if e != nil {
		return e
	}
	r.divCursor = 1
	r.Div = Int{}
	r.DivActive = false
	r.divCause = nil
	return nil
}

// Comparison owns actual RAW mpq tmp1/tmp2. Body exceptions skip source del;
// normal completion destroys tmp1 NUM/DEN before tmp2 NUM/DEN.
func (m *RationalManager) finishRawCompare(e *error) {
	r := &m.RawArithmetic
	if *e != nil {
		r.compareCause = *e
		if r.CompareOne.Num.Cell == 0 && r.CompareOne.Den.Cell == 0 && r.CompareTwo.Num.Cell == 0 && r.CompareTwo.Den.Cell == 0 {
			r.CompareActive = false
			r.CompareOne = Rational{}
			r.CompareTwo = Rational{}
		}
	}
}
func (r *rationalRawArithmeticScope) compareField() *Int {
	switch r.compareCursor {
	case 0:
		return &r.CompareOne.Num
	case 1:
		return &r.CompareOne.Den
	case 2:
		return &r.CompareTwo.Num
	default:
		return &r.CompareTwo.Den
	}
}
func (m *RationalManager) rawCompareEnd() error {
	r := &m.RawArithmetic
	r.compareEnd = true
	for r.compareCursor < 4 {
		guard, e := sourceNumeralDel(m.Z, r.compareField())
		r.compareGuard = guard
		if e != nil {
			return e
		}
		r.compareCursor++
	}
	r.CompareOne = Rational{}
	r.CompareTwo = Rational{}
	r.CompareActive = false
	r.compareCause = nil
	return nil
}
func (m *RationalManager) finishRawPower(e *error) {
	r := &m.RawArithmetic
	if *e != nil {
		r.powerCause = *e
		if r.Power.Num.Cell == 0 && r.Power.Den.Cell == 0 {
			r.PowerActive = false
			r.Power = Rational{}
		}
	}
}
func (m *RationalManager) finishRawAdd(e *error) {
	r := &m.RawArithmetic
	if *e != nil {
		r.addCause = *e
		if r.AddOne.Cell == 0 && r.AddTwo.Cell == 0 {
			r.AddActive = false
			r.AddOne = Int{}
			r.AddTwo = Int{}
		}
	}
}
func (m *RationalManager) finishRawNormalize(e *error) {
	r := &m.RawArithmetic
	if *e != nil {
		r.normCause = *e
		if r.Norm.Cell == 0 {
			r.NormActive = false
			r.Norm = Int{}
		}
	}
}
func (m *RationalManager) rawPowerEnd() error {
	r := &m.RawArithmetic
	r.powerEnd = true
	for r.powerCursor < 2 {
		p := &r.Power.Num
		if r.powerCursor == 1 {
			p = &r.Power.Den
		}
		guard, e := sourceNumeralDel(m.Z, p)
		r.powerGuard = guard
		if e != nil {
			return e
		}
		r.powerCursor++
	}
	r.Power = Rational{}
	r.PowerActive = false
	r.powerCause = nil
	return nil
}
func (m *RationalManager) rawAddEnd() error {
	r := &m.RawArithmetic
	r.addEnd = true
	for r.addCursor < 2 {
		p := &r.AddOne
		if r.addCursor == 1 {
			p = &r.AddTwo
		}
		guard, e := sourceNumeralDel(m.Z, p)
		r.addGuard = guard
		if e != nil {
			return e
		}
		r.addCursor++
	}
	r.AddOne = Int{}
	r.AddTwo = Int{}
	r.AddActive = false
	r.addCause = nil
	return nil
}
func (m *RationalManager) rawNormalizeEnd() error {
	r := &m.RawArithmetic
	r.normEnd = true
	guard, e := sourceNumeralDel(m.Z, &r.Norm)
	r.normGuard = guard
	if e != nil {
		return e
	}
	r.Norm = Int{}
	r.NormActive = false
	r.normCause = nil
	return nil
}

// Explicit owning cleanup acknowledges reached numeric release progress. This
// is not source exception unwinding: RAW mpq/mpz have no destructors.
// Concrete identities belong to actual started RAW groups, not numeral values.
// The counter lives outside the resettable raw scope so a later foreign call
// cannot reuse a completed continuation's identity. No source allocator event.
type rationalRawGroup uint8

const (
	rationalRawNormalize rationalRawGroup = iota
	rationalRawDivision
	rationalRawComparison
	rationalRawAddition
	RationalRawPower
)

func (m *RationalManager) beginRawGeneration() (uint64, error) {
	if m.RawGeneration == ^uint64(0) {
		return 0, ErrRationalRawArithmeticPending
	}
	m.RawGeneration++
	return m.RawGeneration, nil
}
func (m *RationalManager) CleanupRawGroup(group rationalRawGroup) error {
	r := &m.RawArithmetic
	if group == rationalRawNormalize && r.NormActive {
		if e := m.Z.Del(&r.Norm); e != nil {
			r.normCause = e
			return e
		}
		r.NormActive = false
	}
	if group == rationalRawDivision && r.DivActive {
		if e := m.Z.Del(&r.Div); e != nil {
			r.divCause = e
			return e
		}
		r.divCursor = 1
		r.DivActive = false
	}

	if group == rationalRawComparison && r.CompareActive {
		for r.compareCursor < 4 {
			if e := m.Z.Del(r.compareField()); e != nil {
				r.compareCause = e
				return e
			}
			r.compareCursor++
		}
		r.CompareActive = false
	}
	if group == rationalRawAddition && r.AddActive {
		for r.addCursor < 2 {
			p := &r.AddOne
			if r.addCursor == 1 {
				p = &r.AddTwo
			}
			if e := m.Z.Del(p); e != nil {
				r.addCause = e
				return e
			}
			r.addCursor++
		}
		r.AddActive = false
	}
	if group == RationalRawPower && r.PowerActive {
		for r.powerCursor < 2 {
			p := &r.Power.Num
			if r.powerCursor == 1 {
				p = &r.Power.Den
			}
			if e := m.Z.Del(p); e != nil {
				r.powerCause = e
				return e
			}
			r.powerCursor++
		}
		r.PowerActive = false
	}
	return nil
}
func (m *RationalManager) cleanupRawArithmetic() error {
	// Preserve the preexisting generic owning adapter's exact group order.
	for _, group := range [...]rationalRawGroup{rationalRawNormalize, rationalRawDivision, rationalRawComparison, rationalRawAddition, RationalRawPower} {
		if e := m.CleanupRawGroup(group); e != nil {
			return e
		}
	}
	m.RawArithmetic = rationalRawArithmeticScope{}
	return nil
}
func (m *RationalManager) CleanupRawArithmetic() error {
	if e := m.checkOwnerReady(); e != nil {
		return e
	}
	return m.cleanupRawArithmetic()
}
