package numeral

// Source: src/util/mpbq.cpp.

import "errors"

var ErrBinarySelectPending = errors.New("mpnum: binary rational selection local cleanup pending")
var ErrBinaryRefinePending = errors.New("mpnum: binary rational refinement midpoint cleanup pending")

const (
	selectRationalLower uint8 = iota + 1
	selectRationalUpper
	selectRationalBoth
)

func (m *BinaryRationalManager) beginSelectRational(qm *RationalManager, shape uint8) error {
	if e := m.open(); e != nil {
		return e
	}
	m.selectRationalOwner = qm
	m.selectRationalShape = shape
	m.selectRationalCursor = 0
	m.selectRationalLower = NewRational()
	m.selectRationalUpper = NewRational()
	m.selectRationalPending = true
	m.selectRationalExecuting = true
	return nil
}

func (m *BinaryRationalManager) selectRationalCount() uint8 {
	if m.selectRationalShape == selectRationalBoth {
		return 4
	}
	return 2
}

func (m *BinaryRationalManager) selectRationalPart() *Int {
	cursor := m.selectRationalCursor
	value := &m.selectRationalLower
	if m.selectRationalShape == selectRationalUpper || m.selectRationalShape == selectRationalBoth && cursor < 2 {
		value = &m.selectRationalUpper
	}
	if cursor%2 == 0 {
		return &value.Num
	}
	return &value.Den
}

func (m *BinaryRationalManager) clearSelectRational() {
	m.selectRationalLower = Rational{}
	m.selectRationalUpper = Rational{}
	m.selectRationalOwner = nil
	m.selectRationalPending = false
}

func (m *BinaryRationalManager) finishSelectRationalSource() error {
	if !m.selectRationalPending {
		return nil
	}
	if m.selectIntPending {
		return ErrBinarySelectPending
	}
	for m.selectRationalCursor < m.selectRationalCount() {
		if _, e := sourceNumeralDel(m.selectRationalOwner.Z, m.selectRationalPart()); e != nil {
			return e
		}
		m.selectRationalCursor++
	}
	m.clearSelectRational()
	return nil
}

// CleanupSelectRationalTemporaries resumes the source's upper-before-lower
// rational destructor order and each rational's numerator-before-denominator.
func (m *BinaryRationalManager) CleanupSelectRationalTemporaries() error {
	if m == nil || m.self != m {
		return ErrBinaryOwner
	}
	if !m.selectRationalPending {
		return nil
	}
	if m.selectRationalExecuting {
		return ErrBinarySelectPending
	}
	if e := m.CleanupSelectIntegerTemporary(); e != nil {
		return e
	}
	if m.selectRationalOwner == nil || m.selectRationalOwner.self != m.selectRationalOwner || m.selectRationalOwner.Z == nil {
		return ErrBinaryOwner
	}
	for m.selectRationalCursor < m.selectRationalCount() {
		if e := m.selectRationalOwner.Z.Del(m.selectRationalPart()); e != nil {
			return e
		}
		m.selectRationalCursor++
	}
	m.clearSelectRational()
	return nil
}

func (m *BinaryRationalManager) selectInteger(lower, upper *BinaryRational, r *Int) (bool, error) {
	if lower.KField == 0 {
		return true, m.Z.Copy(r, &lower.Num)
	}
	if upper.KField == 0 {
		return true, m.Z.Copy(r, &upper.Num)
	}
	if e := m.Ceil(lower, &m.SelectIntTmp1); e != nil {
		return false, e
	}
	if e := m.Floor(upper, &m.SelectIntTmp2); e != nil {
		return false, e
	}
	yes, e := m.Z.LessEqual(&m.SelectIntTmp1, &m.SelectIntTmp2)
	if e != nil || !yes {
		return false, e
	}
	return true, m.Z.Copy(r, &m.SelectIntTmp1)
}
func (m *BinaryRationalManager) rationalIntegerBound(qm *RationalManager, q *Rational, out *Int, upper bool) (err error) {
	if isIntegerRational(q) {
		if e := m.Z.CopyFrom(out, qm.Z, &q.Num); e != nil {
			return e
		}
		if upper {
			return m.Z.Dec(out)
		}
		return m.Z.Inc(out)
	}
	if e := m.open(); e != nil {
		return e
	}
	m.selectIntOwner = qm
	m.selectIntTemporary = Int{}
	m.selectIntPending = true
	m.selectIntExecuting = true
	defer func() {
		if e := m.finishSelectIntegerTemporarySource(); err == nil {
			err = e
		}
		m.selectIntExecuting = false
	}()
	tmp := &m.selectIntTemporary
	var e error
	if upper {
		e = qm.FloorToInt(q, tmp)
	} else {
		e = qm.CeilToInt(q, tmp)
	}
	if e != nil {
		return e
	}
	return m.Z.CopyFrom(out, qm.Z, tmp)
}

func (m *BinaryRationalManager) finishSelectIntegerTemporarySource() error {
	if !m.selectIntPending {
		return nil
	}
	if _, e := sourceNumeralDel(m.selectIntOwner.Z, &m.selectIntTemporary); e != nil {
		return e
	}
	m.selectIntPending = false
	m.selectIntOwner = nil
	return nil
}

// CleanupSelectIntegerTemporary completes a stopped scoped_mpz destructor
// through its borrowed rational manager before another selection or close.
func (m *BinaryRationalManager) CleanupSelectIntegerTemporary() error {
	if m == nil || m.self != m {
		return ErrBinaryOwner
	}
	if !m.selectIntPending {
		return nil
	}
	if m.selectIntExecuting {
		return ErrBinarySelectPending
	}
	if m.selectIntOwner == nil || m.selectIntOwner.self != m.selectIntOwner || m.selectIntOwner.Z == nil {
		return ErrBinaryOwner
	}
	if e := m.selectIntOwner.Z.Del(&m.selectIntTemporary); e != nil {
		return e
	}
	m.selectIntPending = false
	m.selectIntOwner = nil
	return nil
}
func (m *BinaryRationalManager) selectIntegerRationalLower(qm *RationalManager, lower *Rational, upper *BinaryRational, r *Int) (bool, error) {
	if upper.KField == 0 {
		return true, m.Z.Copy(r, &upper.Num)
	}
	if e := m.rationalIntegerBound(qm, lower, &m.SelectIntTmp1, false); e != nil {
		return false, e
	}
	if e := m.Floor(upper, &m.SelectIntTmp2); e != nil {
		return false, e
	}
	yes, e := m.Z.LessEqual(&m.SelectIntTmp1, &m.SelectIntTmp2)
	if e != nil || !yes {
		return false, e
	}
	return true, m.Z.Copy(r, &m.SelectIntTmp1)
}
func (m *BinaryRationalManager) selectIntegerRationalUpper(qm *RationalManager, lower *BinaryRational, upper *Rational, r *Int) (bool, error) {
	if lower.KField == 0 {
		return true, m.Z.Copy(r, &lower.Num)
	}
	if e := m.Ceil(lower, &m.SelectIntTmp1); e != nil {
		return false, e
	}
	if e := m.rationalIntegerBound(qm, upper, &m.SelectIntTmp2, true); e != nil {
		return false, e
	}
	yes, e := m.Z.LessEqual(&m.SelectIntTmp1, &m.SelectIntTmp2)
	if e != nil || !yes {
		return false, e
	}
	return true, m.Z.Copy(r, &m.SelectIntTmp1)
}
func (m *BinaryRationalManager) selectIntegerRationalBounds(qm *RationalManager, lower, upper *Rational, r *Int) (bool, error) {
	if e := m.rationalIntegerBound(qm, lower, &m.SelectIntTmp1, false); e != nil {
		return false, e
	}
	if e := m.rationalIntegerBound(qm, upper, &m.SelectIntTmp2, true); e != nil {
		return false, e
	}
	yes, e := m.Z.LessEqual(&m.SelectIntTmp1, &m.SelectIntTmp2)
	if e != nil || !yes {
		return false, e
	}
	return true, m.Z.Copy(r, &m.SelectIntTmp1)
}
func (m *BinaryRationalManager) SelectSmall(lower, upper, r *BinaryRational) (bool, error) {
	if e := m.triple(lower, upper, r); e != nil {
		return false, e
	}
	bad, e := m.Greater(lower, upper)
	if e != nil || bad {
		return false, e
	}
	return true, m.SelectSmallCore(lower, upper, r)
}
func (m *BinaryRationalManager) SelectSmallCore(lower, upper, r *BinaryRational) error {
	if e := m.triple(lower, upper, r); e != nil {
		return e
	}
	yes, e := m.LessEqual(lower, upper)
	if e != nil {
		return e
	}
	if !yes {
		return ErrBinaryInterval
	}
	aux := &m.SelectSmallTmp
	yes, e = m.selectInteger(lower, upper, aux)
	if e != nil {
		return e
	}
	if yes {
		return m.SetInteger(r, aux)
	}
	minK, maxK := uint32(0), lower.KField
	if upper.KField < maxK {
		maxK = upper.KField
	}
	l2k, u2k := &m.SelectSmallTmp1, &m.SelectSmallTmp2
	if maxK <= 8 {
		k := uint32(0)
		if e = m.Copy(l2k, lower); e != nil {
			return e
		}
		if e = m.Copy(u2k, upper); e != nil {
			return e
		}
		for {
			k++
			if e = m.Mul2(l2k); e != nil {
				return e
			}
			if e = m.Mul2(u2k); e != nil {
				return e
			}
			yes, e = m.selectInteger(l2k, u2k, aux)
			if e != nil {
				return e
			}
			if yes {
				if e = m.SetIntegerPower(r, aux, k); e != nil {
					return e
				}
				break
			}
		}
	} else {
		for {
			mid := minK + (maxK-minK)/2
			if e = m.Copy(l2k, lower); e != nil {
				return e
			}
			if e = m.Copy(u2k, upper); e != nil {
				return e
			}
			if e = m.Mul2k(l2k, mid); e != nil {
				return e
			}
			if e = m.Mul2k(u2k, mid); e != nil {
				return e
			}
			yes, e = m.selectInteger(l2k, u2k, aux)
			if e != nil {
				return e
			}
			if yes {
				maxK = mid
			} else {
				minK = mid + 1
			}
			if minK == maxK {
				if maxK != mid {
					if e = m.Copy(l2k, lower); e != nil {
						return e
					}
					if e = m.Copy(u2k, upper); e != nil {
						return e
					}
					if e = m.Mul2k(l2k, maxK); e != nil {
						return e
					}
					if e = m.Mul2k(u2k, maxK); e != nil {
						return e
					}
					yes, e = m.selectInteger(l2k, u2k, aux)
					if e != nil {
						return e
					}
					if !yes {
						return ErrBinaryInvariant
					}
				}
				if e = m.SetIntegerPower(r, aux, maxK); e != nil {
					return e
				}
				break
			}
		}
	}
	return m.debugSelected(lower, upper, r)
}
func (m *BinaryRationalManager) SelectSmallRationalLower(qm *RationalManager, lower *Rational, upper, r *BinaryRational) (err error) {
	if e := m.pair(upper, r); e != nil {
		return e
	}
	if qm == nil {
		return ErrBinaryOwner
	}
	if e := qm.check(lower); e != nil {
		return e
	}
	m.traceSelectSmallRationalLower(qm, lower, upper)
	yes, e := m.GreaterRational(upper, qm, lower)
	if e != nil {
		return e
	}
	if !yes {
		return ErrBinaryInterval
	}
	aux := &m.SelectSmallTmp
	yes, e = m.selectIntegerRationalLower(qm, lower, upper, aux)
	if e != nil {
		return e
	}
	if yes {
		return m.SetInteger(r, aux)
	}
	if e = m.beginSelectRational(qm, selectRationalLower); e != nil {
		return e
	}
	defer func() {
		if cleanup := m.finishSelectRationalSource(); err == nil {
			err = cleanup
		}
		m.selectRationalExecuting = false
	}()
	l2k := &m.selectRationalLower
	two := Rational{Num: NewInt(2), Den: NewInt(1)}
	u2k := &m.SelectSmallTmp2
	if e = qm.Copy(l2k, lower); e != nil {
		return e
	}
	if e = m.Copy(u2k, upper); e != nil {
		return e
	}
	for k := uint32(1); ; k++ {
		if e = qm.Mul(l2k, &two, l2k); e != nil {
			return e
		}
		if e = m.Mul2(u2k); e != nil {
			return e
		}
		yes, e = m.selectIntegerRationalLower(qm, l2k, u2k, aux)
		if e != nil {
			return e
		}
		if yes {
			return m.SetIntegerPower(r, aux, k)
		}
	}
}
func (m *BinaryRationalManager) SelectSmallRationalUpper(qm *RationalManager, lower *BinaryRational, upper *Rational, r *BinaryRational) (err error) {
	if e := m.pair(lower, r); e != nil {
		return e
	}
	if qm == nil {
		return ErrBinaryOwner
	}
	if e := qm.check(upper); e != nil {
		return e
	}
	yes, e := m.LessRational(lower, qm, upper)
	if e != nil {
		return e
	}
	if !yes {
		return ErrBinaryInterval
	}
	aux := &m.SelectSmallTmp
	yes, e = m.selectIntegerRationalUpper(qm, lower, upper, aux)
	if e != nil {
		return e
	}
	if yes {
		return m.SetInteger(r, aux)
	}
	if e = m.beginSelectRational(qm, selectRationalUpper); e != nil {
		return e
	}
	defer func() {
		if cleanup := m.finishSelectRationalSource(); err == nil {
			err = cleanup
		}
		m.selectRationalExecuting = false
	}()
	u2k := &m.selectRationalUpper
	two := Rational{Num: NewInt(2), Den: NewInt(1)}
	l2k := &m.SelectSmallTmp2
	if e = m.Copy(l2k, lower); e != nil {
		return e
	}
	if e = qm.Copy(u2k, upper); e != nil {
		return e
	}
	for k := uint32(1); ; k++ {
		if e = m.Mul2(l2k); e != nil {
			return e
		}
		if e = qm.Mul(u2k, &two, u2k); e != nil {
			return e
		}
		yes, e = m.selectIntegerRationalUpper(qm, l2k, u2k, aux)
		if e != nil {
			return e
		}
		if yes {
			return m.SetIntegerPower(r, aux, k)
		}
	}
}
func (m *BinaryRationalManager) SelectSmallRationalBounds(qm *RationalManager, lower, upper *Rational, r *BinaryRational) (err error) {
	if e := m.check(r); e != nil {
		return e
	}
	if qm == nil {
		return ErrBinaryOwner
	}
	if e := qm.check(lower); e != nil {
		return e
	}
	if e := qm.check(upper); e != nil {
		return e
	}
	cmp, e := qm.Compare(lower, upper)
	yes := cmp < 0
	if e != nil {
		return e
	}
	if !yes {
		return ErrBinaryInterval
	}
	aux := &m.SelectSmallTmp
	yes, e = m.selectIntegerRationalBounds(qm, lower, upper, aux)
	if e != nil {
		return e
	}
	if yes {
		return m.SetInteger(r, aux)
	}
	if e = m.beginSelectRational(qm, selectRationalBoth); e != nil {
		return e
	}
	defer func() {
		if cleanup := m.finishSelectRationalSource(); err == nil {
			err = cleanup
		}
		m.selectRationalExecuting = false
	}()
	l2k, u2k := &m.selectRationalLower, &m.selectRationalUpper
	two := Rational{Num: NewInt(2), Den: NewInt(1)}
	if e = qm.Copy(l2k, lower); e != nil {
		return e
	}
	if e = qm.Copy(u2k, upper); e != nil {
		return e
	}
	for k := uint32(1); ; k++ {
		if e = qm.Mul(l2k, &two, l2k); e != nil {
			return e
		}
		if e = qm.Mul(u2k, &two, u2k); e != nil {
			return e
		}
		yes, e = m.selectIntegerRationalBounds(qm, l2k, u2k, aux)
		if e != nil {
			return e
		}
		if yes {
			return m.SetIntegerPower(r, aux, k)
		}
	}
}
func (m *BinaryRationalManager) RefineUpper(qm *RationalManager, q *Rational, l, u *BinaryRational) error {
	return m.refine(qm, q, l, u, true)
}
func (m *BinaryRationalManager) RefineLower(qm *RationalManager, q *Rational, l, u *BinaryRational) error {
	return m.refine(qm, q, l, u, false)
}
func (m *BinaryRationalManager) refine(qm *RationalManager, q *Rational, l, u *BinaryRational, upper bool) (err error) {
	return m.refineWithReceipt(qm, q, l, u, upper, nil)
}
func (m *BinaryRationalManager) refineWithReceipt(qm *RationalManager, q *Rational, l, u *BinaryRational, upper bool, receipt *BinaryRefineInvocationReceipt) (err error) {
	if e := m.pair(l, u); e != nil {
		return e
	}
	if m.refineInvocation == ^uint64(0) {
		return ErrBinaryRefinePending
	}
	m.refineInvocation++
	m.refineSourceCause = nil
	m.refineDebugPending = false
	if receipt != nil {
		*receipt = BinaryRefineInvocationReceipt{m, m.refineInvocation}
	}
	m.refineImported = NewRational()
	m.refineImportedCursor = 0
	m.refineImportedPending = true
	m.refineExecuting = true
	defer func() {
		if cleanup := m.cleanupRefineImported(); cleanup != nil {
			m.refineSourceCause = cleanup
			if err == nil {
				err = cleanup
			}
		}
		m.refineDebugPending = m.toOwnedRationalScope.pending || m.debugIntegerPending || m.debugArithmeticPending || len(m.debugSnapshotQueue) != 0
		m.refineDebugChildren.capture(m)
		if m.refineSourceCause == nil {
			m.refineSourceCause = m.toOwnedRationalScope.sourceCause
		}
		if m.refineSourceCause == nil && m.debugArithmeticPending {
			m.refineSourceCause = m.debugArithmeticCall.SourceReady()
		}
		if (m.debugIntegerPending || len(m.debugSnapshotQueue) != 0) && m.refineSourceCause == nil {
			m.refineSourceCause = err
		}
		m.refineExecuting = false
	}()
	input, e := m.importRational(qm, q, &m.refineImported)
	if e != nil {
		return e
	}
	lo, e := m.LessRational(l, qm, q)
	if e != nil {
		return e
	}
	if !lo {
		return ErrBinaryInterval
	}
	hi, e := m.GreaterRational(u, qm, q)
	if e != nil {
		return e
	}
	if !lo || !hi {
		return ErrBinaryInterval
	}
	power, _, e := m.Z.IsPowerOfTwo(&input.Den)
	if e != nil {
		return e
	}
	if power {
		return ErrBinaryInterval
	}
	m.refineMid = BinaryRational{}
	m.refinePending = true
	defer func() {
		if err != nil && m.refineMid.Num.Cell == 0 {
			m.refinePending = false
			m.refineMid = BinaryRational{}
		}
	}()
	mid := &m.refineMid
	for {
		if e = m.Add(l, u, mid); e != nil {
			return e
		}
		if e = m.Div2(mid); e != nil {
			return e
		}
		var stop bool
		if upper {
			stop, e = m.GreaterRational(mid, qm, q)
		} else {
			stop, e = m.LessRational(mid, qm, q)
		}
		if e != nil {
			return e
		}
		if upper == stop {
			if e = m.Swap(u, mid); e != nil {
				return e
			}
		} else {
			if e = m.Swap(l, mid); e != nil {
				return e
			}
		}
		if stop {
			if e = m.Del(mid); e != nil {
				return e
			}
			m.refineMid = BinaryRational{}
			m.refinePending = false
			return m.debugEnclosure(qm, q, l, u)
		}
	}
}

func (m *BinaryRationalManager) cleanupRefineImported() error {
	if !m.refineImportedPending {
		return nil
	}
	roots := [2]*Int{&m.refineImported.Num, &m.refineImported.Den}
	for m.refineImportedCursor < 2 {
		if e := m.Z.Del(roots[m.refineImportedCursor]); e != nil {
			return e
		}
		m.refineImportedCursor++
	}
	m.refineImported = Rational{}
	m.refineImportedCursor = 0
	m.refineImportedPending = false
	return nil
}

// CleanupRefineImported resumes release of the Go cross-owner rational copy.
func (m *BinaryRationalManager) CleanupRefineImported() error {
	if m == nil || m.self != m {
		return ErrBinaryOwner
	}
	if m.refineExecuting {
		return ErrBinaryRefinePending
	}
	return m.cleanupRefineImported()
}

// CleanupRefineMid owns the raw midpoint left by a source body failure or a
// stopped explicit del(mid); it never replays a completed source deletion.
func (m *BinaryRationalManager) CleanupRefineMid() error {
	if m == nil || m.self != m {
		return ErrBinaryOwner
	}
	if !m.refinePending {
		return nil
	}
	if m.refineExecuting {
		return ErrBinaryRefinePending
	}
	if e := m.Z.Del(&m.refineMid.Num); e != nil {
		return e
	}
	m.refineMid = BinaryRational{}
	m.refinePending = false
	return nil
}
func (m *BinaryRationalManager) Approx(a *BinaryRational, k uint32, plus bool) (err error) {
	if e := m.check(a); e != nil {
		return e
	}
	if a.KField <= k {
		return nil
	}
	var old BinaryRational
	defer func() {
		if cleanup := m.debugApproxDrop(&old); err == nil {
			err = cleanup
		}
	}()
	if e := m.debugApproxSnapshot(a, &old); e != nil {
		return e
	}
	neg := a.Num.Val < 0
	shift := a.KField - k
	if e := m.Z.Abs(&a.Num); e != nil {
		return e
	}
	if e := m.Z.MachineDiv2k(&a.Num, shift, &a.Num); e != nil {
		return e
	}
	if neg != plus {
		if e := m.Z.Inc(&a.Num); e != nil {
			return e
		}
	}
	if neg {
		if e := m.Z.Neg(&a.Num); e != nil {
			return e
		}
	}
	a.KField = k
	if e := m.normalize(a); e != nil {
		return e
	}
	return m.debugApprox(&old, a, plus)
}

// CleanupApproxDebug resumes the retained scoped_mpbq snapshot's deletion.
func (m *BinaryRationalManager) CleanupApproxDebug() error {
	if m == nil || m.self != m {
		return ErrBinaryOwner
	}
	if !m.approxPending {
		return nil
	}
	if e := m.Z.Del(&m.approxOld.Num); e != nil {
		return e
	}
	m.approxOld = BinaryRational{}
	m.approxPending = false
	return nil
}

func (m *BinaryRationalManager) ApproxDivDefault(a, b, c *BinaryRational) error {
	return m.ApproxDiv(a, b, c, 32, false)
}
func (m *BinaryRationalManager) ApproxDiv(a, b, c *BinaryRational, k uint32, plus bool) error {
	if e := m.triple(a, b, c); e != nil {
		return e
	}
	if b.Num.Val == 0 {
		return ErrDivideByZero
	}
	power, kp, e := m.Z.IsPowerOfTwo(&b.Num)
	if e != nil {
		return e
	}
	if power {
		if e = m.debugAssert(b.KField == 0 || kp == 0); e != nil {
			return e
		}
		if e = m.Z.Copy(&c.Num, &a.Num); e != nil {
			return e
		}
		if b.KField > 0 {
			if e = m.debugAssert(kp == 0); e != nil {
				return e
			}
			two := NewInt(2)
			if e = m.Z.Power(&two, b.KField, &m.DivTmp1); e != nil {
				return e
			}
			if e = m.Z.Mul(&c.Num, &m.DivTmp1, &c.Num); e != nil {
				return e
			}
		}
		c.KField = a.KField + kp
		return m.normalize(c)
	}
	divides, e := m.Z.Divides(&b.Num, &a.Num)
	if e != nil {
		return e
	}
	if divides {
		if e = m.Z.Div(&a.Num, &b.Num, &c.Num); e != nil {
			return e
		}
		if a.KField >= b.KField {
			c.KField = a.KField - b.KField
		} else {
			if e = m.Z.Mul2k(&c.Num, b.KField-a.KField, &c.Num); e != nil {
				return e
			}
			c.KField = 0
		}
		return m.normalize(c)
	}
	neg := (a.Num.Val < 0) != (b.Num.Val < 0)
	absA, normA, absB := &m.DivTmp1, &m.DivTmp2, &m.DivTmp3
	if e = m.Z.Copy(absA, &a.Num); e != nil {
		return e
	}
	if e = m.Z.Abs(absA); e != nil {
		return e
	}
	if e = m.Z.Copy(absB, &b.Num); e != nil {
		return e
	}
	if e = m.Z.Abs(absB); e != nil {
		return e
	}
	if a.KField > b.KField {
		if k >= a.KField-b.KField {
			e = m.Z.Mul2k(absA, k-(a.KField-b.KField), normA)
		} else {
			e = m.Z.MachineDiv2k(absA, (a.KField-b.KField)-k, normA)
		}
	} else {
		e = m.Z.Mul2k(absA, k+b.KField-a.KField, normA)
	}
	if e != nil {
		return e
	}
	c.KField = k
	if e = m.Z.Div(normA, absB, &c.Num); e != nil {
		return e
	}
	if neg != plus {
		if e = m.Z.Inc(&c.Num); e != nil {
			return e
		}
	}
	if neg {
		if e = m.Z.Neg(&c.Num); e != nil {
			return e
		}
	}
	return m.normalize(c)
}
