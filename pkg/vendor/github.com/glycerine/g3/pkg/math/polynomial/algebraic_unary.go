package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:1086-1159,1326-1381.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"math"
)

func (m *AlgebraicRootCore) makeUnary(a, out *Algebraic, k uint32, root bool) (cause error) {
	s := &m.binary
	if s.active {
		return ErrAlgebraicPending
	}
	if e := m.assertBinaryRoot(a, 1088, "!a.is_basic()"); e != nil {
		return e
	}
	if e := algebraicQueryTagAssertionSwitch(); e != nil {
		return e
	}
	*s = algebraicBinaryScope{polynomialCaller: m.polynomialCaller, active: true, running: true}
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
		cause = errors.Join(cause, cleanup)
		s.running = false
		if cause == nil {
			s.active = false
		}
	}()
	p, im := m.config.Polynomials, m.config.Intervals
	var e error
	s.p, e = p.NewPolynomial()
	if e != nil {
		return e
	}
	s.f, e = p.NewPolynomial()
	if e != nil {
		return e
	}
	if e = m.unaryResultPolynomial(a, k, s.p, root); e != nil {
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
		if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", 1104, "seqs.size() == num_fs"); e != nil {
			return e
		}
	}
	if e = m.beginSavedInterval(&s.saved[0], a); e != nil {
		return e
	}
	s.intervalConstructed = true
	for {
		if e = m.checkpoint(); e != nil {
			return e
		}
		if e = m.assertBinaryRoot(a, 1111, "!a.is_basic()"); e != nil {
			return e
		}
		la, e := m.location(a)
		if e != nil {
			return e
		}
		ra, e := m.load(la)
		if e != nil {
			return e
		}
		s.executing = true
		if root {
			_, e = m.config.Binary.RootLowerWithInvocation(ra.interval.BorrowLower(), k, s.interval.BorrowLower(), &s.intervalRoots[0])
			if e == nil {
				_, e = m.config.Binary.RootUpperWithInvocation(ra.interval.BorrowUpper(), k, s.interval.BorrowUpper(), &s.intervalRoots[1])
			}
		} else {
			e = im.PowerWithDebugReceipt(&ra.interval, k, &s.interval, &s.intervalDebug)
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
			lv, e := p.SignVariationsBinary(seq, s.interval.BorrowLower())
			if e != nil {
				return e
			}
			uv, e := p.SignVariationsBinary(seq, s.interval.BorrowUpper())
			if e != nil {
				return e
			}
			if e = m.traceUnaryIntervals(a, lv, uv); e != nil {
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
			if e = m.restoreBinaryInterval(&s.saved[0]); e != nil {
				return e
			}
			factor, e := s.factors.FactorView(target)
			if e != nil {
				return e
			}
			if e = p.SetView(s.f, factor); e != nil {
				return e
			}
			return m.setBinaryCore(out, s.sequences[target], targetLower, targetUpper, fullFactorization)
		}
		refinable, e := m.refine(a)
		if e != nil {
			return e
		}
		if !refinable {
			d, enabled, e := algebraicAssertionSwitch()
			if e != nil {
				return e
			}
			if enabled {
				l, e := m.location(a)
				if e != nil {
					return e
				}
				if l != nil && l.kind != 0 {
					if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", 1152, "a.is_basic()"); e != nil {
						return e
					}
				}
			}
			if e = m.restoreBinaryInterval(&s.saved[0]); e != nil {
				return e
			}
			return m.dispatchUnary(a, k, out, root)
		}
	}
}
