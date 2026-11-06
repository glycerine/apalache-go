package numeral

// Source: src/util/mpq.cpp:312-315; src/util/mpf.cpp:254-270.

import (
	"errors"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// This route is the same floor/prev-power body with its real scoped NUM in
// the reached caller frame. The source can invoke it once in either branch.
func (q *RationalManager) prevPowerOfTwoWithRationalSetContext(c integerAllocationInvocationContext) (value uint32, cause error) {
	if q == nil || q.self != q || q.Z == nil {
		return 0, umemory.ErrRuntimeOwner
	}
	s := c.rationalSetFrame(q.Z, true)
	if s == nil || s.manager.rational != q {
		return 0, umemory.ErrRuntimeOwner
	}
	if s.prevLive {
		return 0, ErrNumericPending
	}
	s.prev = Int{}
	s.prevLive = true
	defer func() {
		// A younger Floor RAW source stop leaves this NUM unread.
		if source := s.manager.rationalSetChildrenSource(s); source != nil {
			s.prevSourceCause = source
			cause = errors.Join(cause, source)
			return
		}
		if _, e := sourceNumeralDel(q.Z, &s.prev); e != nil {
			s.prevSourceCause = e
			cause = errors.Join(cause, e)
			return
		}
		s.prevLive = false
	}()
	if e := q.roundToIntWithAllocationContext(&s.sig, &s.prev, false, c); e != nil {
		return 0, e
	}
	return q.Z.PrevPowerOfTwo(&s.prev)
}
