package polynomial

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type upolySSFrame struct {
	lower, upper     unumeral.BinaryRational
	lowerSV, upperSV uint32
}
type upolySSStack struct {
	values []upolySSFrame
}

func numericBinaryRationalManager_pushSS(b *unumeral.BinaryRationalManager, s *upolySSStack) (*upolySSFrame, error) {
	if uint64(len(s.values)) >= math.MaxUint32 || len(s.values) == int(^uint(0)>>1) {
		return nil, arena.ErrBounds
	}
	old := s.values
	s.values = append(s.values, upolySSFrame{})
	if len(old) != 0 && &old[0] != &s.values[0] {
		clear(old)
	}
	return &s.values[len(s.values)-1], nil
}
func numericBinaryRationalManager_popSS(b *unumeral.BinaryRationalManager, s *upolySSStack) error {
	if uint32(len(s.values)) == 0 {
		return arena.ErrBounds
	}
	f := &s.values[uint32(len(s.values))-1]
	if e := unumeral.SourceIntegerDel(b.Integers(), &f.lower.Num); e != nil {
		return e
	}
	if e := unumeral.SourceIntegerDel(b.Integers(), &f.upper.Num); e != nil {
		return e
	}
	s.values = s.values[:len(s.values)-1]
	return nil
}
func numericBinaryRationalManager_dropSS(b *unumeral.BinaryRationalManager, s *upolySSStack) {
	for i := uint32(0); i < uint32(len(s.values)); i++ {
		b.Del(&s.values[i].lower)
		b.Del(&s.values[i].upper)
	}
	*s = upolySSStack{}
}
func numericBinaryRationalManager_addSSInterval(b *unumeral.BinaryRationalManager, lower, upper *unumeral.BinaryRational, l, u *BinaryRationalVector) error {
	if _, e := numericBinaryRationalManager_binaryPushDefault(b, &l.state); e != nil {
		return e
	}
	if _, e := numericBinaryRationalManager_binaryPushDefault(b, &u.state); e != nil {
		return e
	}
	if e := b.Copy(&l.state.values[l.state.size-1], lower); e != nil {
		return e
	}
	return b.Copy(&u.state.values[u.state.size-1], upper)
}
func (m *UPolynomialManager) pushSSFrame(v UPolynomialView, b *unumeral.BinaryRationalManager, s *upolySSStack, r, l, u *BinaryRationalVector, lowerSV, upperSV uint32, lower, upper *unumeral.BinaryRational, sourceLine int32) error {
	return m.pushSSFrameWithIsolation(v, b, s, r, l, u, lowerSV, upperSV, lower, upper, sourceLine, nil)
}
func (m *UPolynomialManager) pushSSFrameWithIsolation(v UPolynomialView, b *unumeral.BinaryRationalManager, s *upolySSStack, r, l, u *BinaryRationalVector, lowerSV, upperSV uint32, lower, upper *unumeral.BinaryRational, sourceLine int32, scope *upolySturmIsolationScope) error {
	if scope != nil && (!scope.entryValid() || scope.binary != b || scope.owner != m || s != &scope.stack) {
		return ErrUPolynomialOwner
	}
	if enabled, e := refinementSourceAssertionsEnabled(); e != nil {
		return e
	} else if enabled {
		same, e := b.Equal(lower, upper)
		if e != nil {
			return e
		}
		if same {
			expression := "!bqm.eq(curr_lower, zero)"
			switch sourceLine {
			case 2442:
				expression = "!bqm.eq(zero, curr_upper)"
			case 2463:
				expression = "!bqm.eq(curr_lower, mid)"
			case 2464:
				expression = "!bqm.eq(mid, curr_upper)"
			}
			if e := refinementSourceAssertionFailure(sourceLine, expression); e != nil {
				return e
			}
		}
	}
	if lowerSV == upperSV {
		return nil
	}
	if lowerSV == upperSV+1 {
		var receipt *upolyBinaryEvaluationInvocationReceipt
		if scope != nil {
			receipt = &scope.endpoint
		}
		sign, e := m.evalSignBinaryWithNegScopeAndReceipt(v, upper, nil, receipt)
		if e != nil {
			return e
		}
		if sign == 0 {
			if _, e = numericBinaryRationalManager_binaryPushDefault(b, &r.state); e != nil {
				return e
			}
			if scope != nil {
				return scope.copyBound(&r.state.values[r.state.size-1], upper)
			}
			return b.Copy(&r.state.values[r.state.size-1], upper)
		}
		if scope == nil {
			return numericBinaryRationalManager_addSSInterval(b, lower, upper, l, u)
		}
		if _, e := numericBinaryRationalManager_binaryPushDefault(b, &l.state); e != nil {
			return e
		}
		if _, e := numericBinaryRationalManager_binaryPushDefault(b, &u.state); e != nil {
			return e
		}
		if e := scope.copyBound(&l.state.values[l.state.size-1], lower); e != nil {
			return e
		}
		return scope.copyBound(&u.state.values[u.state.size-1], upper)
	}
	f, e := numericBinaryRationalManager_pushSS(b, s)
	if e != nil {
		return e
	}
	if scope != nil {
		e = scope.copyBound(&f.lower, lower)
	} else {
		e = b.Copy(&f.lower, lower)
	}
	if e != nil {
		return e
	}
	if scope != nil {
		e = scope.copyBound(&f.upper, upper)
	} else {
		e = b.Copy(&f.upper, upper)
	}
	if e != nil {
		return e
	}
	f.lowerSV = lowerSV
	f.upperSV = upperSV
	return nil
}
func (m *UPolynomialManager) SturmIsolateRootsWithBounds(v UPolynomialView, neg, pos uint32, b *unumeral.BinaryRationalManager, r, l, u *BinaryRationalVector) (cause error) {
	if e := m.isolationOwners(b, r, l, u); e != nil {
		return e
	}
	if e := m.sturmIsolationNoZeroAssertion(v); e != nil {
		return e
	}
	q, e := m.beginSturmIsolation(v, b)
	if e != nil {
		return e
	}
	defer func() { cause = q.finish(cause) }()
	return q.isolate(v, neg, pos, b, r, l, u)
}
func (q *upolySturmIsolationScope) isolate(v UPolynomialView, neg, pos uint32, b *unumeral.BinaryRationalManager, r, l, u *BinaryRationalVector) error {
	if !q.entryValid() {
		return ErrUPolynomialOwner
	}
	m := q.owner
	q.constructCore()
	seq := &q.sequence.state
	mid, lower, upper := &q.mid, &q.lower, &q.upper
	if e := q.sequence.buildSturm(v, UPolynomialView{}, 0); e != nil {
		return e
	}
	if e := q.constructStack(); e != nil {
		return e
	}
	s := &q.stack
	if e := q.traceInitial(v); e != nil {
		return e
	}
	minus, e := m.sequenceVariations(seq, 1, nil)
	if e != nil {
		return e
	}
	zeroSV, e := m.sequenceVariations(seq, 2, nil)
	if e != nil {
		return e
	}
	plus, e := m.sequenceVariations(seq, 0, nil)
	if e != nil {
		return e
	}
	if plus >= minus {
		return nil
	}
	ops := b.WithAllocationContext(q.context().utility(m.config.Integer))
	if e = ops.PowerOfTwo(neg, lower, &q.copy); e != nil {
		return e
	}
	if e = ops.Neg(lower); e != nil {
		return e
	}
	if e = ops.PowerOfTwo(pos, upper, &q.copy); e != nil {
		return e
	}
	zero := unumeral.NewBinaryInt(0)
	if e = m.pushSSFrameWithIsolation(v, b, s, r, l, u, minus, zeroSV, lower, &zero, 2441, q); e != nil {
		return e
	}
	if e = m.pushSSFrameWithIsolation(v, b, s, r, l, u, zeroSV, plus, &zero, upper, 2442, q); e != nil {
		return e
	}
	for uint32(len(s.values)) != 0 {
		if e = m.checkpoint(); e != nil {
			return e
		}
		f := &s.values[uint32(len(s.values))-1]
		minus, plus = f.lowerSV, f.upperSV
		if e = b.Swap(lower, &f.lower); e != nil {
			return e
		}
		if e = b.Swap(upper, &f.upper); e != nil {
			return e
		}
		if e = numericBinaryRationalManager_popSS(b, s); e != nil {
			q.childStop = e
			return e
		}
		if e = refinementSourceAssert(minus > plus+1, 2452, "lower_sv > upper_sv + 1"); e != nil {
			return e
		}
		if e = ops.AddSub(lower, upper, mid, false); e != nil {
			return e
		}
		if e = ops.Div2(mid); e != nil {
			return e
		}
		if e := q.traceMidpoint(minus, plus); e != nil {
			return e
		}
		midSV, e := q.sequence.signVariationsBinary(mid)
		if e != nil {
			return e
		}
		if e = m.pushSSFrameWithIsolation(v, b, s, r, l, u, minus, midSV, lower, mid, 2463, q); e != nil {
			return e
		}
		if e = m.pushSSFrameWithIsolation(v, b, s, r, l, u, midSV, plus, mid, upper, 2464, q); e != nil {
			return e
		}
	}
	return nil
}
func (m *UPolynomialManager) SturmIsolateRoots(v UPolynomialView, b *unumeral.BinaryRationalManager, r, l, u *BinaryRationalVector) (cause error) {
	if e := m.isolationOwners(b, r, l, u); e != nil {
		return e
	}
	if e := refinementSourceAssert(l.state.size == u.state.size, 2469, "lowers.size() == uppers.size()"); e != nil {
		return e
	}
	q, e := m.beginSturmIsolation(v, b)
	if e != nil {
		return e
	}
	defer func() { cause = q.finish(cause) }()
	if e := q.constructPrepared(v); e != nil {
		return e
	}
	p := &q.prepared
	if e := m.normalizeWithSourceContext(m.stateView(p), q.context()); e != nil {
		return e
	}
	pos, e := m.KnuthPositiveRootUpperBound(v)
	if e != nil {
		return e
	}
	neg, e := m.knuthNegativeRootUpperBoundWithReceipt(v, &q.knuth)
	if e != nil {
		return e
	}
	if e := m.sturmIsolationNoZeroAssertion(m.stateView(p)); e != nil {
		return e
	}
	return q.isolate(m.stateView(p), neg, pos, b, r, l, u)
}

// The operand is inside SASSERT and must not execute in erased/disabled builds.
func (m *UPolynomialManager) sturmIsolationNoZeroAssertion(v UPolynomialView) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	zero, e := m.HasZeroRoots(v)
	if e != nil {
		return e
	}
	if zero {
		return refinementSourceAssertionFailure(2394, "!has_zero_roots(sz, p)")
	}
	return nil
}
