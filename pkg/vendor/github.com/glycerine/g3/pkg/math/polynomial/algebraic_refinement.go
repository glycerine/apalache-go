package polynomial

// algebraic_numbers.cpp:243–335,703–781,897–929.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type algebraicPredicateScope struct {
	trace                                                                                      algebraicPredicateTraceInvocation
	setter                                                                                     algebraicIntegerSetInvocation
	integerComparison                                                                          unumeral.IntegerCopyAllocationReceipt
	integerEvaluation                                                                          upolyIntegerEvaluationInvocationReceipt
	numericOwner                                                                               *AlgebraicRootCore
	numericHooks                                                                               unumeral.NumericInvocationHooks
	numericRoots                                                                               unumeral.NumericAllocationRoots
	arithmeticExecuting                                                                        bool
	candidateCopy                                                                              unumeral.IntegerCopyAllocationReceipt
	sourceStop                                                                                 error
	savedSourceCursor, savedOwnCursor                                                          uint8
	leadingCopy                                                                                unumeral.IntegerCopyAllocationReceipt
	savedCopy                                                                                  unumeral.IntegerCopyAllocationReceipt
	refinement                                                                                 algebraicRefinementInvocation
	childStop, ownCause                                                                        error
	ownStage                                                                                   uint8
	finishing                                                                                  bool
	saved                                                                                      unumeral.BinaryInterval
	owner                                                                                      Algebraic
	lower, upper                                                                               unumeral.BinaryRational
	candidate                                                                                  unumeral.Int
	active, savedActive, savedConstructed, restored, candidateActive, lowerActive, upperActive bool
	stage                                                                                      uint8
	cause, cleanup                                                                             error
}

func (m *AlgebraicRootCore) magnitude(l, u *unumeral.BinaryRational) (int32, error) {
	return m.magnitudeWithCleanup(l, u, false)
}
func (m *AlgebraicRootCore) magnitudeForCleanup(l, u *unumeral.BinaryRational) (int32, error) {
	return m.magnitudeWithCleanup(l, u, true)
}
func (m *AlgebraicRootCore) magnitudeWithCleanup(l, u *unumeral.BinaryRational, cleanup bool) (int32, error) {
	b := m.config.Binary
	z := m.config.Rationals.Integers()
	if e := m.magnitudeSignAssertWithCleanup(l, u, cleanup); e != nil {
		return 0, e
	}
	if l.K() == u.K() {
		if cleanup {
			return b.MagnitudeUpperForCleanup(l)
		}
		return b.MagnitudeUpper(l)
	}
	var positive bool
	var e error
	if cleanup {
		positive, e = b.IsNonNegForCleanup(l)
	} else {
		positive, e = b.IsNonNeg(l)
	}
	if e != nil {
		return 0, e
	}
	var x, y uint32
	if positive {
		x, e = z.Log2(&(*u.BorrowNumerator()))
		if e != nil {
			return 0, e
		}
		y, e = z.Log2(&(*l.BorrowNumerator()))
	} else {
		x, e = z.MLog2(&(*u.BorrowNumerator()))
		if e != nil {
			return 0, e
		}
		y, e = z.MLog2(&(*l.BorrowNumerator()))
	}
	if e != nil {
		return 0, e
	}
	// Source starts with unsigned log2, so each signed k participates after
	// unsigned32 promotion. Final uint32-to-int conversion uses the selected
	// int32 signed-bit profile, including representable extreme exponents.
	return int32(x - y - u.K() + l.K() - u.K()), nil
}
func (m *AlgebraicRootCore) refine(a *Algebraic) (bool, error) { return m.refineAt(a, 0, false) }
func (m *AlgebraicRootCore) refineN(a *Algebraic, k uint32) (bool, error) {
	for i := uint32(0); i < k; i++ {
		ok, e := m.refine(a)
		if e != nil || !ok {
			return ok, e
		}
	}
	return true, nil
}
func (m *AlgebraicRootCore) Refine(a *Algebraic) error { _, e := m.refine(a); return e }
func (m *AlgebraicRootCore) RefineToPrecision(a *Algebraic, k uint32) error {
	_, e := m.refineAt(a, k, true)
	return e
}
func (m *AlgebraicRootCore) refineAt(a *Algebraic, k uint32, precision bool) (bool, error) {
	return m.refineAtWithReceipt(a, k, precision, nil)
}
func (m *AlgebraicRootCore) refineAtWithReceipt(a *Algebraic, k uint32, precision bool, receipt *algebraicRefinementInvocation) (bool, error) {
	if m != nil && m.self == m && (m.refinement.pending(m) || m.refinementConversion.pending(m) || m.refinementQ.active) {
		return false, ErrAlgebraicPending
	}
	l, e := m.location(a)
	if e != nil {
		return false, e
	}
	if l == nil || l.kind == 0 {
		return precision, nil
	}
	// Both refine and refine_until_prec reach anum::to_algebraic only
	// after the basic tag short circuit (cpp748,770; header373).
	if e = algebraicQueryTagAssertionSwitch(); e != nil {
		return false, e
	}
	r, e := m.load(l)
	if e != nil {
		return false, e
	}
	v, e := m.sourcePolynomial(&r)
	if e != nil {
		return false, e
	}
	s := 1
	if r.flags&2 != 0 {
		s = -1
	}
	ok, e := m.refineRecordWithReceipt(l, &r, v, s, k, precision, receipt)
	if e != nil {
		return false, e
	}
	if ok {
		return true, nil
	}
	return false, m.promoteRefinedRoot(a, &r)
}

// The exact-root branch after refineRecord has published its interval.
func (m *AlgebraicRootCore) promoteRefinedRoot(a *Algebraic, r *algebraicRecord) error {
	if e := m.beginRefinementQ(); e != nil {
		return e
	}
	if e := m.refinementConversion.run(m, r.interval.BorrowLower(), &m.localQ); e != nil {
		return m.finishRefinementQ(e)
	}
	if e := m.Del(a); e != nil {
		return m.finishRefinementQ(e)
	}
	e := m.SetTake(a, &m.localQ)
	return m.finishRefinementQ(e)
}
func (m *AlgebraicRootCore) beginPredicate() error {
	if m.predicate.active {
		return ErrAlgebraicPending
	}
	m.predicate = algebraicPredicateScope{active: true}
	return nil
}
func (m *AlgebraicRootCore) restoreSaved() error {
	p := &m.predicate
	if !p.savedActive || !p.savedConstructed || p.restored {
		return nil
	}
	p.restored = true
	l, e := m.locationCleanup(&p.owner)
	if e != nil {
		return e
	}
	if l == nil || l.kind == 0 {
		return nil
	}
	if e = algebraicQueryTagAssertionSwitch(); e != nil {
		return e
	}
	r, e := m.load(l)
	if e != nil {
		return e
	}
	mag, e := m.magnitude(&(*r.interval.BorrowLower()), &(*r.interval.BorrowUpper()))
	if e != nil {
		return e
	}
	if mag < m.minMagnitude {
		e = m.config.Intervals.Swap(&r.interval, &p.saved)
		return m.save(l, &r, e)
	}
	return nil
}
func (m *AlgebraicRootCore) finishPredicate(cause error) error {
	p := &m.predicate
	if !p.active {
		return cause
	}
	if p.finishing || p.arithmeticExecuting || m.predicateQ.executing || m.predicateDiagnosticRunning {
		return errors.Join(cause, ErrAlgebraicPending)
	}
	p.finishing = true
	defer func() { p.finishing = false }()
	if p.cause == nil {
		p.cause = cause
	}
	if p.childStop != nil || p.sourceStop != nil {
		return errors.Join(cause, p.childStop, p.sourceStop)
	}
	// Actual scoped locals destruct in reverse declaration order. A failed saved
	// constructor has no source destructor; its partial fields get owning cleanup.
	for p.stage < 5 {
		var e error
		switch p.stage {
		case 0:
			if p.candidateActive {
				e = unumeral.SourceIntegerDel(m.config.Rationals.Integers(), &p.candidate)
			}
		case 1:
			if p.upperActive {
				e = unumeral.SourceIntegerDel(m.config.Rationals.Integers(), p.upper.BorrowNumerator())
			}
		case 2:
			if p.lowerActive {
				e = unumeral.SourceIntegerDel(m.config.Rationals.Integers(), p.lower.BorrowNumerator())
			}
		case 3:
			e = m.restoreSaved()
		case 4:
			if p.savedActive && p.savedConstructed {
				fields := [2]*unumeral.Int{p.saved.BorrowLower().BorrowNumerator(), p.saved.BorrowUpper().BorrowNumerator()}
				for p.savedSourceCursor < 2 {
					e = unumeral.SourceIntegerDel(m.config.Rationals.Integers(), fields[p.savedSourceCursor])
					if e != nil {
						break
					}
					p.savedSourceCursor++
				}
			}
		}
		if e != nil {
			p.cleanup = e
			p.sourceStop = e
			return errors.Join(cause, e)
		}
		p.stage++
	}
	if p.savedActive && !p.savedConstructed {
		return cause
	}
	z := m.config.Rationals.Integers()
	if m.predicateTracePending(&p.trace) || m.integerSetterPending(&p.setter) || m.predicateQ.active || p.integerEvaluationPending(m) || z.MatchesCopyAllocationInvocation(&p.integerComparison) || unumeral.NumericAllocationPending(&p.numericRoots) || z.MatchesCopyAllocationInvocation(&p.savedCopy) || z.MatchesCopyAllocationInvocation(&p.leadingCopy) || z.MatchesCopyAllocationInvocation(&p.candidateCopy) {
		return cause
	}
	m.predicate = algebraicPredicateScope{}
	return cause
}
func (m *AlgebraicRootCore) IsInt(a *Algebraic) (result bool, err error) {
	l, err := m.location(a)
	if err != nil {
		return false, err
	}
	if l == nil || l.kind == 0 {
		// basic_value asserts even for the canonical null zero (cpp236).
		if err = algebraicQueryTagAssertionSwitch(); err != nil {
			return false, err
		}
		if l == nil {
			return true, nil
		}
		if err = algebraicQueryTagAssertionSwitch(); err != nil {
			return false, err
		}
		// mpq::is_int reaches only denominator.is_one; mpz::is_one
		// short circuits on a nonnull cell before reading m_val.
		lo, e := m.config.Allocator.ReadUint32(l.raw, 16)
		if e != nil {
			return false, e
		}
		hi, e := m.config.Allocator.ReadUint32(l.raw, 20)
		if e != nil || lo != 0 || hi != 0 {
			return false, e
		}
		value, e := m.config.Allocator.ReadUint32(l.raw, 12)
		return value == 1, e
	}
	if err = algebraicQueryTagAssertionSwitch(); err != nil {
		return false, err
	}
	flags, err := m.config.Allocator.ReadUint32(l.raw, 16)
	if err != nil || flags&4 != 0 {
		return false, err
	}
	if m.predicate.active {
		return false, ErrAlgebraicPending
	}
	ok, err := m.refineAt(a, 1, true)
	if err != nil {
		return false, err
	}
	if !ok {
		if err = m.predicateBasicAssert(a, 251); err != nil {
			return false, err
		}
		return m.IsInt(a)
	}
	l, err = m.location(a)
	if err != nil {
		return false, err
	}
	r, err := m.load(l)
	if err != nil {
		return false, err
	}
	if err = m.beginPredicate(); err != nil {
		return false, err
	}
	defer func() { err = m.finishPredicate(err) }()
	p := &m.predicate
	if err = m.floorPredicateCandidate(r.interval.BorrowUpper()); err != nil {
		return false, err
	}
	if err = m.predicateUpperAssert(r.interval.BorrowUpper(), false); err != nil {
		return false, err
	}

	less, err := m.lessPredicateInteger(r.interval.BorrowLower())
	if err != nil || !less {
		return false, err
	}
	v, err := m.sourcePolynomial(&r)
	if err != nil {
		return false, err
	}
	s, err := m.evalPredicateInteger(v)
	if err != nil || s != 0 {
		return false, err
	}
	err = m.setIntegerWithInvocation(a, &p.candidate, &p.setter)
	if m.matchesIntegerSetter(&p.setter) && m.integerSet.active {
		p.childStop = m.integerSet.sourceStop
	}
	return err == nil, err
}

func (m *AlgebraicRootCore) IsRational(a *Algebraic) (result bool, err error) {
	return m.isRationalWithSourceStop(a, nil)
}

// Report only this actual call's source stop to its lexical caller. The core
// retains ownership of predicate work; the caller does not acquire its cleanup.
func (m *AlgebraicRootCore) isRationalWithSourceStop(a *Algebraic, sourceStop *error) (result bool, err error) {
	l, err := m.location(a)
	if err != nil {
		return false, err
	}
	if l == nil || l.kind == 0 {
		return true, nil
	}
	if err = algebraicQueryTagAssertionSwitch(); err != nil {
		return false, err
	}
	flags, err := m.config.Allocator.ReadUint32(l.raw, 16)
	if err != nil || flags&4 != 0 {
		return false, err
	}
	r, err := m.load(l)
	if err != nil {
		return false, err
	}
	if m.predicate.active {
		return false, ErrAlgebraicPending
	}
	traceBefore := m.predicateTrace.invocation
	if err = m.traceRationalPredicate(a, 0, 0); err != nil {
		// Phase zero precedes predicate construction. Only a newly accepted trace
		// call can report its source stop; an older refused frame is unrelated.
		trace := &m.predicateTrace
		if sourceStop != nil && trace.invocation != traceBefore && trace.owner == m && trace.active {
			*sourceStop = trace.stop
		}
		return false, err
	}
	if err = m.beginPredicate(); err != nil {
		return false, err
	}
	// cpp292 passes c, implicitly creating temporary anum (header366),
	// whose const& in save_intervals cpp897–929 dangles. Go explicitly binds
	// this actual caller for safe ownership; this is not defined C++ cleanup.
	defer func() {
		m.predicate.owner = *a
		err = m.finishPredicate(err)
		if sourceStop != nil {
			*sourceStop = errors.Join(m.predicate.childStop, m.predicate.sourceStop)
		}
	}()
	p := &m.predicate
	if err = m.constructPredicateSaved(a, &r); err != nil {
		return false, err
	}
	v, err := m.sourcePolynomial(&r)
	if err != nil {
		return false, err
	}
	lead, err := m.config.Polynomials.read(v, r.size-1)
	if err != nil {
		return false, err
	}
	z := m.config.Rationals.Integers()
	if err = z.CopyFromOwnerWithReceipt(&m.isRationalTmp, z, lead, &p.leadingCopy); err != nil {
		return false, err
	}
	if err = z.Abs(&m.isRationalTmp); err != nil {
		return false, err
	}
	k, err := z.Log2(&m.isRationalTmp)
	if err != nil {
		return false, err
	}
	k++
	if err = m.traceRationalPredicate(a, 1, k); err != nil {
		return false, err
	}
	ok, err := m.refineAtWithReceipt(a, k, true, &p.refinement)
	if err != nil {
		p.childStop = m.refinementSourceStop(&p.refinement)
	}
	if err != nil {
		return false, err
	}
	if !ok {
		if err = m.predicateBasicAssert(a, 306); err != nil {
			return false, err
		}
		return true, nil
	}
	if err = m.traceRationalPredicate(a, 2, k); err != nil {
		return false, err
	}
	l, err = m.location(a)
	if err != nil {
		return false, err
	}
	r, err = m.load(l)
	if err != nil {
		return false, err
	}
	if err = m.scalePredicateBounds(&r); err != nil {
		return false, err
	}
	if err = m.floorPredicateCandidate(&p.upper); err != nil {
		return false, err
	}
	if err = m.beginPredicateQ(); err != nil {
		return false, err
	}
	defer func() { err = m.finishPredicateQ(err) }()
	if err = m.setPredicateQParts(&p.candidate, &m.isRationalTmp); err != nil {
		return false, err
	}
	if err = m.predicateUpperAssert(r.interval.BorrowUpper(), true); err != nil {
		return false, err
	}

	less, err := m.lessPredicateRational(r.interval.BorrowLower())
	if err != nil {
		return false, err
	}
	if less {
		v, err = m.sourcePolynomial(&r)
		if err != nil {
			return false, err
		}
		s, e := m.evalPredicateRational(v)
		if e != nil {
			return false, e
		}
		if s == 0 {
			if err = m.restoreSaved(); err != nil {
				return false, err
			}
			err = m.SetTake(a, &m.localQ)
			return err == nil, err
		}
	}
	if err = m.restoreSaved(); err != nil {
		return false, err
	}
	l, err = m.location(a)
	if err != nil {
		return false, err
	}
	r, err = m.load(l)
	if err != nil {
		return false, err
	}
	r.flags |= 4
	return false, m.store(l, &r)
}

// Store every interval publication, including partial body/error results,
// before releasing the executing span. Rational-root promotion follows it.
func (m *AlgebraicRootCore) refineRecord(l *algebraicLocation, r *algebraicRecord, v UPolynomialView, sign int, k uint32, precision bool) (ok bool, cause error) {
	return m.refineRecordWithReceipt(l, r, v, sign, k, precision, nil)
}
func (m *AlgebraicRootCore) refineRecordWithReceipt(l *algebraicLocation, r *algebraicRecord, v UPolynomialView, sign int, k uint32, precision bool, receipt *algebraicRefinementInvocation) (ok bool, cause error) {
	s := &m.refinement
	if e := s.begin(m); e != nil {
		return false, e
	}
	if receipt != nil {
		*receipt = algebraicRefinementInvocation{m, s, s.invocation}
	}
	defer func() { s.executing = false }()
	p, b := m.config.Polynomials, m.config.Binary
	if precision {
		// cpp:773 recomputes the sign via refine, unlike refine_core(c):730.
		ok, cause = p.refineToPrecisionWithEvaluationReceipt(v, b, r.interval.BorrowLower(), r.interval.BorrowUpper(), k, &s.evaluation, s)
	} else {
		ok, cause = p.refineCoreWithEvaluationReceipt(v, sign, b, r.interval.BorrowLower(), r.interval.BorrowUpper(), &s.evaluation, s)
	}
	return ok, m.save(l, r, cause)
}

// Promotion's source scoped_mpq destructor is separate from owning cleanup.
func (m *AlgebraicRootCore) finishRefinementQ(cause error) error {
	return m.refinementQ.finish(m, cause)
}

// Concrete retained predicate declaration phases. No source reservation is
// added by these helpers; each active flag follows its actual construction.
func (m *AlgebraicRootCore) constructPredicateSaved(a *Algebraic, r *algebraicRecord) error {
	p := &m.predicate
	p.savedActive = true
	p.owner = *a
	if e := m.config.Intervals.CopyWithAllocationReceipt(&p.saved, &r.interval, &p.savedCopy); e != nil {
		return e
	}
	p.savedConstructed = true
	return nil
}
func (m *AlgebraicRootCore) scalePredicateBounds(r *algebraicRecord) error {
	p := &m.predicate
	if e := p.beginArithmetic(m); e != nil {
		return e
	}
	defer func() { p.arithmeticExecuting = false }()
	ops := m.config.Binary.WithAllocationContext(p.numericContext())
	p.lowerActive = true
	p.upperActive = true
	if e := ops.MulInteger(r.interval.BorrowLower(), &m.isRationalTmp, &p.lower); e != nil {
		return e
	}
	return ops.MulInteger(r.interval.BorrowUpper(), &m.isRationalTmp, &p.upper)
}
func (m *AlgebraicRootCore) floorPredicateCandidate(upper *unumeral.BinaryRational) error {
	p := &m.predicate
	if e := p.beginArithmetic(m); e != nil {
		return e
	}
	defer func() { p.arithmeticExecuting = false }()
	p.candidateActive = true
	return m.config.Binary.WithAllocationContext(p.numericContext()).FloorWithCopyAllocationReceipt(upper, &p.candidate, &p.candidateCopy)
}
