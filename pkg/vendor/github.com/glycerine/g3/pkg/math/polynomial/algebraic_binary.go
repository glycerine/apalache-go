package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:844-889,897-1090,
// 1440-1494,1544-1563. Concrete addition/subtraction/multiplication routes.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type algebraicSavedInterval struct {
	copy                         unumeral.IntegerCopyAllocationReceipt
	target                       *Algebraic
	value                        unumeral.BinaryInterval
	owned, constructed, restored bool
}
type algebraicBinaryScope struct {
	squareFree                           upolySquareFreeCallReceipt
	conversion                           algebraicConversionScope
	intervalRoots                        [2]unumeral.BinaryRootInvocation
	intervalDebug                        unumeral.BinaryIntervalDebugInvocationReceipt
	owner                                *AlgebraicRootCore
	numericHooks                         unumeral.NumericInvocationHooks
	numericBefore                        unumeral.NumericAllocationRoots
	numericRoots                         unumeral.NumericAllocationRoots
	traceRootPending                     bool
	traceRootStop                        error
	refinable                            upolyRefinableInvocationReceipt
	polynomialCaller                     *polynomialAlgebraicEvaluationScope
	p, f                                 UPolynomial
	factors                              *UPolynomialFactors
	factorReceipt                        upolyFactorFrontendInvocationReceipt
	sequences                            []*UPolynomialSequence
	pendingSequence                      *UPolynomialSequence
	saved                                [2]algebraicSavedInterval
	interval                             unumeral.BinaryInterval
	intervalConstructed                  bool
	localQ                               unumeral.Rational
	qConstructed                         bool
	sequenceCursor                       int
	active, running, executing, cleaning bool
	sourceStop                           error
}

func (s *algebraicBinaryScope) NumericBound(executing bool) bool {
	m := s.owner
	return m != nil && m.self == m && !m.closed && s == &m.binary && s.active && (!executing || s.executing)
}
func (s *algebraicBinaryScope) NumericRoots(z *unumeral.IntegerManager, local *unumeral.NumericAllocationRoots) *unumeral.NumericAllocationRoots {
	if !s.NumericBound(true) || s.owner.config.Rationals.Integers() != z {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.numericRoots
}
func (s *algebraicBinaryScope) numericContext() unumeral.NumericAllocationContext {
	return unumeral.NumericAllocationContext{}.WithCaller(&s.numericHooks)
}

// Add, Sub, and Mul follow the source dispatch and root selector. Inputs and
// output may alias; selection can refine input intervals. CleanupRetained
// completes a stopped owning scope before another binary operation.
func (m *AlgebraicRootCore) Add(a, b, out *Algebraic) error { return m.arithmeticBinary(a, b, out, 0) }
func (m *AlgebraicRootCore) Sub(a, b, out *Algebraic) error { return m.arithmeticBinary(a, b, out, 1) }
func (m *AlgebraicRootCore) Mul(a, b, out *Algebraic) error { return m.arithmeticBinary(a, b, out, 2) }
func (m *AlgebraicRootCore) arithmeticBinary(a, b, out *Algebraic, kind uint8) error {
	if m != nil && m.self == m && m.binary.active {
		return ErrAlgebraicPending
	}
	return m.dispatchBinary(a, b, out, kind)
}
func (m *AlgebraicRootCore) dispatchBinary(a, b, out *Algebraic, kind uint8) error {
	la, e := m.location(a)
	if e != nil {
		return e
	}
	if kind == 2 && la == nil {
		return m.Del(out)
	}
	lb, e := m.location(b)
	if e != nil {
		return e
	}
	if _, e = m.location(out); e != nil {
		return e
	}
	if kind == 2 && (la == nil || lb == nil) {
		return m.Del(out)
	}
	if la == nil {
		if e = m.Copy(out, b); e != nil {
			return e
		}
		if kind == 1 {
			return m.Neg(out)
		}
		return nil
	}
	if lb == nil {
		return m.Copy(out, a)
	}
	if lb.kind == 0 {
		r, e := m.load(lb)
		if e != nil {
			return e
		}
		switch kind {
		case 0:
			return m.AddRational(a, &r.value, out)
		case 1:
			return m.SubRational(a, &r.value, out)
		default:
			return m.MulRational(a, &r.value, out)
		}
	}
	if la.kind == 0 {
		r, e := m.load(la)
		if e != nil {
			return e
		}
		if kind == 2 {
			return m.MulRational(b, &r.value, out)
		}
		if kind == 0 {
			return m.AddRational(b, &r.value, out)
		}
		if e = m.SubRational(b, &r.value, out); e != nil {
			return e
		}
		return m.Neg(out)
	}
	return m.makeBinary(a, b, out, kind)
}

func (m *AlgebraicRootCore) restoreBinaryInterval(saved *algebraicSavedInterval) error {
	saved.restored = true
	l, e := m.locationCleanup(saved.target)
	if e != nil || l == nil || l.kind == 0 {
		return e
	}
	if e = algebraicQueryTagAssertionSwitch(); e != nil {
		return e
	}
	r, e := m.load(l)
	if e != nil {
		return e
	}
	magnitude, e := m.magnitudeForCleanup(r.interval.BorrowLower(), r.interval.BorrowUpper())
	if e != nil {
		return e
	}
	if magnitude >= m.minMagnitude {
		return nil
	}
	e = m.config.Intervals.SwapForCleanup(&r.interval, &saved.value)
	return m.save(l, &r, e)
}

func (m *AlgebraicRootCore) finishBinaryLocals() error {
	s := &m.binary
	if s.conversion.stop != nil {
		return s.conversion.stop
	}
	for i := range s.intervalRoots {
		if s.intervalRoots[i].Pending() {
			if e := s.intervalRoots[i].SourceCause(); e != nil {
				return e
			}
		}
	}
	p := m.config.Polynomials
	if p.squareFreeCallReceiptPending(&s.squareFree) {
		if e := p.squareFreeCall.sourceStop; e != nil {
			return e
		}
		p.squareFreeCall.parentSourceExecuting = true
		defer func() { p.squareFreeCall.parentSourceExecuting = false }()
	}
	if p.defaultNormalizationReceiptPending(&m.coefficientNormalization) {
		if e := p.defaultNormalization.sourceCause; e != nil {
			return e
		}
		p.defaultNormalization.parentSourceExecuting = true
		defer func() { p.defaultNormalization.parentSourceExecuting = false }()
	}
	if m.config.Intervals.DebugInvocationPending(&s.intervalDebug) {
		if e := m.config.Intervals.DebugInvocationSourceCause(&s.intervalDebug); e != nil {
			return e
		}
	}
	if s.traceRootPending && s.traceRootStop != nil {
		return s.traceRootStop
	}
	if e := m.rootTraceSourceCause(); e != nil {
		return e
	}
	if p.refinableReceiptPending(&s.refinable) {
		if e := p.refinableReceiptSourceCause(&s.refinable); e != nil {
			return e
		}
		// Only this actual parent's source unwind may read past an unjoined body child.
		p.refinable.parentSourceExecuting = true
		defer func() { p.refinable.parentSourceExecuting = false }()
	}
	if s.qConstructed {
		if e := m.config.Rationals.Del(&s.localQ); e != nil {
			return e
		}
		s.qConstructed = false
	}
	if s.intervalConstructed {
		if e := m.config.Intervals.Del(&s.interval); e != nil {
			return e
		}
		s.intervalConstructed = false
	}
	for i := 1; i >= 0; i-- {
		if e := m.finishSavedInterval(&s.saved[i], false); e != nil {
			return e
		}
	}
	// scoped_ptr_vector destroys its elements in forward order.
	for s.sequenceCursor < len(s.sequences) {
		seq := s.sequences[s.sequenceCursor]
		if seq != nil {
			if e := seq.Close(); e != nil {
				return e
			}
			s.sequences[s.sequenceCursor] = nil
		}
		s.sequenceCursor++
	}
	if s.factors != nil {
		if e := s.factors.Close(); e != nil {
			return e
		}
		s.factors = nil
	}
	for _, poly := range []*UPolynomial{&s.f, &s.p} {
		if poly.slot != 0 {
			if e := m.config.Polynomials.DelPolynomial(poly); e != nil {
				return e
			}
		}
	}
	return nil
}

func (m *AlgebraicRootCore) cleanupBinary() error {
	s := &m.binary
	if s.running || s.cleaning {
		return ErrAlgebraicPending
	}
	if !s.active {
		return nil
	}
	s.cleaning = true
	defer func() { s.cleaning = false }()
	if e := s.conversion.cleanup(); e != nil {
		return e
	}
	if e := m.config.Polynomials.cleanupSquareFreeCall(&s.squareFree); e != nil {
		return e
	}
	for i := len(s.intervalRoots) - 1; i >= 0; i-- {
		if e := s.intervalRoots[i].Cleanup(false); e != nil {
			return e
		}
	}
	if e := m.config.Intervals.CleanupDebugInvocation(&s.intervalDebug); e != nil {
		return e
	}
	if e := s.numericRoots.Cleanup(m.config.Rationals.Integers()); e != nil {
		return e
	}
	if s.traceRootPending {
		if e := m.cleanupRootDisplay(); e != nil {
			return e
		}
		s.traceRootPending = false
	}
	if e := m.cleanupRootTrace(); e != nil {
		return e
	}
	if e := m.config.Polynomials.cleanupRefinableReceipt(&s.refinable); e != nil {
		return e
	}
	if r := &s.factorReceipt; r.service != nil {
		if e := r.service.cleanupFactorFrontendInvocation(r, false); e != nil {
			return e
		}
	}
	if e := m.finishBinaryLocals(); e != nil {
		return e
	}
	for i := len(s.saved) - 1; i >= 0; i-- {
		if e := m.finishSavedInterval(&s.saved[i], true); e != nil {
			return e
		}
	}
	for i := len(s.intervalRoots) - 1; i >= 0; i-- {
		if e := s.intervalRoots[i].Cleanup(true); e != nil {
			return e
		}
	}
	// A sequence not yet pushed when Sturm construction failed is an unjoined
	// owning allocation, separate from the source pointer-vector destructor.
	if s.pendingSequence != nil {
		if e := s.pendingSequence.Close(); e != nil {
			return e
		}
		s.pendingSequence = nil
	}
	if r := &s.factorReceipt; r.service != nil {
		if e := r.service.cleanupFactorFrontendInvocation(r, true); e != nil {
			return e
		}
	}
	s.active = false
	return nil
}

func (m *AlgebraicRootCore) makeBinary(a, b, out *Algebraic, kind uint8) (cause error) {
	s := &m.binary
	if s.active {
		return ErrAlgebraicPending
	}
	if e := m.assertBinaryRoot(a, 1005, "!a.is_basic()"); e != nil {
		return e
	}
	if e := m.assertBinaryRoot(b, 1006, "!b.is_basic()"); e != nil {
		return e
	}
	if e := algebraicQueryTagAssertionSwitch(); e != nil {
		return e
	}
	if e := algebraicQueryTagAssertionSwitch(); e != nil {
		return e
	}
	generation := s.numericHooks.Generation
	if generation == ^uint64(0) {
		return ErrAlgebraicPending
	}
	numericBefore, e := unumeral.CaptureNumericAllocationRoots(m.config.Rationals.Integers())
	if e != nil {
		return e
	}
	*s = algebraicBinaryScope{polynomialCaller: m.polynomialCaller, owner: m, active: true, running: true}
	s.numericBefore = numericBefore
	s.numericHooks = unumeral.NumericInvocationHooks{Generation: generation + 1, Policy: s}
	defer func() {
		s.executing = false
		var cleanup error
		if m.resultant.active && m.resultant.sourceStop != nil {
			cleanup = m.resultant.sourceStop
		} else if r := &s.factorReceipt; r.service != nil && r.service.matchesFactorFrontendHistory(r) && r.service.frontend.receipt == *r && r.service.frontend.sourceCause != nil {
			cleanup = r.service.frontend.sourceCause
		} else {
			cleanup = m.finishBinaryLocals()
		}
		if cleanup != nil {
			s.sourceStop = cleanup
		}
		captured := s.numericRoots.After(m.config.Rationals.Integers(), s.numericBefore)
		cause = errors.Join(cause, cleanup, captured)
		s.running = false
		if cause == nil {
			s.active = false
		}
	}()
	p, im := m.config.Polynomials, m.config.Intervals
	s.p, e = p.NewPolynomial()
	if e != nil {
		return e
	}
	s.f, e = p.NewPolynomial()
	if e != nil {
		return e
	}
	if e = m.binaryResultPolynomial(a, b, s.p, kind); e != nil {
		return e
	}
	if e = m.traceBinaryOperands(a, b); e != nil {
		return e
	}
	s.factors, e = p.NewFactors()
	if e != nil {
		return e
	}
	v, e := p.wholeView(s.p)
	if e != nil {
		return e
	}
	s.executing = true
	fullFactorization, e := m.factorPolynomialIntoWithSquareFreeReceipt(v, s.factors, &s.squareFree)
	s.executing = false
	// Capture the issued tuple bound to this actual factors owner. A service
	// refusal caused by an older caller conveys no cleanup authority.
	frontend := &m.config.Factors.frontend
	if frontend.result == s.factors && frontend.receipt.invocation != 0 {
		s.factorReceipt = frontend.receipt
	}
	if e != nil {
		return e
	}
	n, e := s.factors.DistinctFactors()
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		if e = m.traceBinaryFactor(i); e != nil {
			return e
		}
		s.pendingSequence, e = p.NewSequence()
		if e != nil {
			return e
		}
		factor, e := s.factors.FactorView(i)
		if e != nil {
			return e
		}
		if e = p.SturmSequence(factor, s.pendingSequence); e != nil {
			return e
		}
		s.sequences = append(s.sequences, s.pendingSequence)
		s.pendingSequence = nil
	}
	d, enabled, e := algebraicAssertionSwitch()
	if e != nil {
		return e
	}
	if enabled && uint32(len(s.sequences)) != n {
		if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", 1024, "seqs.size() == num_fs"); e != nil {
			return e
		}
	}
	for i, target := range []*Algebraic{a, b} {
		if e = m.beginSavedInterval(&s.saved[i], target); e != nil {
			return e
		}
	}
	s.intervalConstructed = true
	for {
		if e = m.checkpoint(); e != nil {
			return e
		}
		if e = m.assertBinaryRoot(a, 1032, "!a.is_basic()"); e != nil {
			return e
		}
		if e = m.assertBinaryRoot(b, 1033, "!b.is_basic()"); e != nil {
			return e
		}
		la, e := m.location(a)
		if e != nil {
			return e
		}
		lb, e := m.location(b)
		if e != nil {
			return e
		}
		ra, e := m.load(la)
		if e != nil {
			return e
		}
		rb, e := m.load(lb)
		if e != nil {
			return e
		}
		s.executing = true
		switch kind {
		case 0:
			e = im.AddWithDebugReceipt(&ra.interval, &rb.interval, &s.interval, &s.intervalDebug)
		case 1:
			e = im.SubWithDebugReceipt(&ra.interval, &rb.interval, &s.interval, &s.intervalDebug)
		case 2:
			e = im.MulWithDebugReceipt(&ra.interval, &rb.interval, &s.interval, &s.intervalDebug)
		}
		s.executing = false
		if e != nil {
			return e
		}
		remaining, target := uint32(0), uint32(math.MaxUint32)
		var targetLower, targetUpper uint32
		for i, seq := range s.sequences {
			if seq == nil {
				continue
			}
			if e = m.traceBinarySequence(uint32(i), seq); e != nil {
				return e
			}
			lv, e := p.SignVariationsBinary(seq, s.interval.BorrowLower())
			if e != nil {
				return e
			}
			uv, e := p.SignVariationsBinary(seq, s.interval.BorrowUpper())
			if e != nil {
				return e
			}
			if e = m.traceBinaryIntervals(a, b, lv, uv); e != nil {
				return e
			}
			if lv <= uv {
				if e = seq.Close(); e != nil {
					return e
				}
				s.sequences[i] = nil
			} else {
				if lv-uv == 1 {
					target, targetLower, targetUpper = uint32(i), lv, uv
				}
				remaining++
			}
		}
		if remaining == 1 && target != math.MaxUint32 {
			if e = m.traceBinaryTarget(target); e != nil {
				return e
			}
			if e = m.restoreBinaryInterval(&s.saved[0]); e != nil {
				return e
			}
			if e = m.restoreBinaryInterval(&s.saved[1]); e != nil {
				return e
			}
			factor, e := s.factors.FactorView(target)
			if e != nil {
				return e
			}
			s.executing = true
			e = p.setViewWithAllocationContext(s.f, factor, s.numericContext())
			s.executing = false
			if e != nil {
				return e
			}
			return m.setBinaryCore(out, s.sequences[target], targetLower, targetUpper, fullFactorization)
		}
		refinable, e := m.refine(a)
		if e != nil {
			return e
		}
		if refinable {
			refinable, e = m.refine(b)
			if e != nil {
				return e
			}
		}
		if !refinable {
			d, enabled, e := algebraicAssertionSwitch()
			if e != nil {
				return e
			}
			if enabled {
				la, e := m.location(a)
				if e != nil {
					return e
				}
				basic := la == nil || la.kind == 0
				if !basic {
					lb, e := m.location(b)
					if e != nil {
						return e
					}
					basic = lb == nil || lb.kind == 0
				}
				if !basic {
					if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", 1078, "a.is_basic() || b.is_basic()"); e != nil {
						return e
					}
				}
			}
			// Preserve the duplicate source call; saved_b is restored at scope exit.
			if e = m.restoreBinaryInterval(&s.saved[0]); e != nil {
				return e
			}
			if e = m.restoreBinaryInterval(&s.saved[0]); e != nil {
				return e
			}
			return m.dispatchBinary(a, b, out, kind)
		}
	}
}

func (m *AlgebraicRootCore) setBinaryCore(out *Algebraic, seq *UPolynomialSequence, lv, uv uint32, minimal bool) error {
	s := &m.binary
	p, im, bm := m.config.Polynomials, m.config.Intervals, m.config.Binary
	v, e := p.wholeView(s.f)
	if e != nil {
		return e
	}
	if e = m.traceSetCorePolynomial(v); e != nil {
		return e
	}
	containsZero, e := im.ContainsZero(&s.interval)
	if e != nil {
		return e
	}
	if containsZero {
		hasZero, e := p.HasZeroRoots(v)
		if e != nil {
			return e
		}
		if hasZero {
			if e = m.Del(out); e != nil {
				return e
			}
			return m.traceSetCoreReset(out)
		}
		zv, e := p.SignVariationsZero(seq)
		if e != nil {
			return e
		}
		var zero unumeral.BinaryRational
		if lv == zv {
			e = im.SetLower(&s.interval, &zero)
		} else {
			d, enabled, e := algebraicAssertionSwitch()
			if e != nil {
				return e
			}
			if enabled && zv != uv {
				if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", 960, "zV == uV"); e != nil {
					return e
				}
			}
			e = im.SetUpper(&s.interval, &zero)
		}
		if e != nil {
			return e
		}
		d, enabled, e := algebraicAssertionSwitch()
		if e != nil {
			return e
		}
		if enabled {
			less, e := bm.Less(s.interval.BorrowLower(), s.interval.BorrowUpper())
			if e != nil {
				return e
			}
			if !less {
				if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", 964, "bqm().lt(r_i.lower(), r_i.upper())"); e != nil {
					return e
				}
			}
		}
	}
	if m.addTmp.slot == 0 {
		m.addTmp, e = p.NewPolynomial()
		if e != nil {
			return e
		}
	}
	hasZero, e := p.HasZeroRoots(v)
	if e != nil {
		return e
	}
	if hasZero {
		e = p.RemoveZeroRoots(v, m.addTmp)
	} else {
		e = p.Swap(s.f, m.addTmp)
	}
	if e != nil {
		return e
	}
	v, e = p.wholeView(m.addTmp)
	if e != nil {
		return e
	}
	refinable, e := p.isolatingToRefinableWithReceipt(v, bm, s.interval.BorrowLower(), s.interval.BorrowUpper(), &s.refinable)
	if e != nil {
		return e
	}
	if !refinable {
		s.localQ, s.qConstructed = unumeral.NewRational(), true
		s.executing = true
		e = s.conversion.run(m, s.interval.BorrowLower(), &s.localQ)
		s.executing = false
		if e != nil {
			return e
		}
		return m.SetTake(out, &s.localQ)
	}
	if e = m.traceSetCoreAlgebraic(); e != nil {
		return e
	}
	return m.SetRoot(out, v, s.interval.BorrowLower(), s.interval.BorrowUpper(), minimal)
}

func algebraicAssertionSwitch() (*diagnostic.DebugContext, bool, error) {
	if !polynomialSourceDebug {
		return nil, false, nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	return d, enabled, e
}
func (m *AlgebraicRootCore) assertBinaryRoot(a *Algebraic, line int32, expression string) error {
	_, enabled, e := algebraicAssertionSwitch()
	if e != nil || !enabled {
		return e
	}
	l, e := m.location(a)
	if e != nil {
		return e
	}
	if l == nil || l.kind == 0 {
		return m.predicateAssertionFailure(line, expression)
	}
	return nil
}
