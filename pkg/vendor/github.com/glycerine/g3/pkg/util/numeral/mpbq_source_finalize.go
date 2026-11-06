package numeral

// Source: src/util/mpbq.cpp:45-57; src/util/basic_interval.h:83-86.

func (m *BinaryRationalManager) checkSourceDestructorAdmission() error {
	if e := m.checkDestructorExecution(); e != nil {
		return e
	}
	if m.Display.active {
		return ErrBinaryDisplayPending
	}
	if m.integerDebugPending || m.convertDebugPending || m.refineDebugPending || m.integerDebugExecuting || m.debugArithmeticExecuting {
		return ErrNumericPending
	}
	return m.open()
}

// SourceFinalize runs the literal scratch-member destructor body. A stopped
// source release is sticky; only CloseChecked owns acknowledgement of a
// consumed physical release and retirement of the Go owner header.
func (m *BinaryRationalManager) SourceFinalize() error {
	if e := m.checkDestructorExecution(); e != nil {
		return e
	}
	if m.sourceStarted {
		return m.sourceCause
	}
	if e := m.checkSourceDestructorAdmission(); e != nil {
		return e
	}
	m.sourceStarted, m.sourceRunning, m.closing = true, true, true
	defer func() { m.sourceRunning = false }()
	roots := [11]*Int{&m.AddMulTmp.Num, &m.Tmp, &m.Tmp2, &m.SelectIntTmp1, &m.SelectIntTmp2, &m.SelectSmallTmp, &m.SelectSmallTmp1.Num, &m.SelectSmallTmp2.Num, &m.DivTmp1, &m.DivTmp2, &m.DivTmp3}
	for m.sourceStage < uint8(len(roots)) {
		if e := SourceIntegerDel(m.Z, roots[m.sourceStage]); e != nil {
			m.sourceCause = e
			return e
		}
		m.sourceStage++
	}
	m.sourceFinalized = true
	return nil
}

// SourceFinalize follows current, maximum, minimum in basic_interval_manager's
// destructor. It does not end the borrowed binary-rational manager.
func (m *BinaryIntervalManager) SourceFinalize() error {
	if m == nil || m.self != m {
		return ErrBinaryOwner
	}
	if m.sourceRunning || m.debugExecuting || m.debugPending {
		return ErrNumericPending
	}
	if m.sourceStarted {
		return m.sourceCause
	}
	if e := m.bounds.checkSourceDestructorAdmission(); e != nil {
		return e
	}
	if e := m.open(); e != nil {
		return e
	}
	m.sourceStarted, m.sourceRunning, m.closing = true, true, true
	defer func() { m.sourceRunning = false }()
	roots := [3]*BinaryRational{&m.MulCurr, &m.MulMax, &m.MulMin}
	for m.sourceStage < uint8(len(roots)) {
		if e := SourceIntegerDel(m.bounds.Z, &roots[m.sourceStage].Num); e != nil {
			m.sourceCause = e
			return e
		}
		m.sourceStage++
	}
	m.sourceFinalized = true
	return nil
}
