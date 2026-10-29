package numeral

// Source: src/util/hwf.cpp:409-427.

import umemory "github.com/glycerine/g3/pkg/util/memory"

func (qm *RationalManager) cleanupHWFConversion(source bool) error {
	if !qm.hwfPending {
		return nil
	}
	parts := [2]*Int{&qm.hwfD, &qm.hwfN}
	for qm.hwfCursor < 2 {
		var e error
		if source {
			_, e = sourceNumeralDel(qm.Z, parts[qm.hwfCursor])
		} else {
			e = qm.Z.Del(parts[qm.hwfCursor])
		}
		if e != nil {
			return e
		}
		qm.hwfCursor++
	}
	qm.hwfN, qm.hwfD = Int{}, Int{}
	qm.hwfPending = false
	qm.hwfCursor = 0
	return nil
}

// CleanupHWFConversion resumes denominator then numerator deletion.
func (qm *RationalManager) CleanupHWFConversion() error {
	if qm == nil || qm.self != qm || qm.Z == nil {
		return umemory.ErrRuntimeOwner
	}
	if qm.hwfExecuting {
		return ErrRationalHWFConversionPending
	}
	return qm.cleanupHWFConversion(false)
}

// ToRational follows the source bit-field decomposition, including its raw
// exponent for subnormals. The destination and scratch integers belong to qm.
func (m *HardwareFloatManager) ToRational(x *HardwareFloat, qm *RationalManager, out *Rational) (err error) {
	if m == nil || x == nil || qm == nil || out == nil {
		return ErrNumericOwner
	}
	if e := qm.checkSetter(out); e != nil {
		return e
	}
	if modularSourceDebug && !(m.IsNormal(x) || m.IsDenormal(x) || m.IsZero(x)) {
		return ErrHardwareFloatNonRegular
	}
	z := qm.Z
	qm.hwfN, qm.hwfD = Int{}, Int{}
	qm.hwfPending = true
	qm.hwfCursor = 0
	qm.hwfExecuting = true
	defer func() {
		if cleanup := qm.cleanupHWFConversion(true); err == nil {
			err = cleanup
		}
		qm.hwfExecuting = false
	}()
	sig := m.Significand(x)
	if m.IsNormal(x) {
		sig |= uint64(1) << 52
	}
	if e := z.SetUint64(&qm.hwfN, sig); e != nil {
		return e
	}
	if m.Sign(x) {
		if e := z.Neg(&qm.hwfN); e != nil {
			return e
		}
	}
	if e := z.SetUint64(&qm.hwfD, uint64(1)<<52); e != nil {
		return e
	}
	exp := m.Exponent(x)
	if exp >= 0 {
		if e := z.Mul2k(&qm.hwfN, uint32(exp), &qm.hwfN); e != nil {
			return e
		}
	} else {
		if e := z.Mul2k(&qm.hwfD, uint32(-exp), &qm.hwfD); e != nil {
			return e
		}
	}
	return qm.SetParts(out, &qm.hwfN, &qm.hwfD)
}
