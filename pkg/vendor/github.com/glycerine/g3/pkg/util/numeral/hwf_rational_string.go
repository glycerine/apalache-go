package numeral

// Source: src/util/hwf.cpp:387-393,409-427;
// src/util/f2n.h:164 instantiated as f2n<hwf_manager>.

import "math/big"

// ToRationalString formats the source's bit-decomposed rational. Go's big.Rat
// normalizes the temporary numerator and denominator before formatting, as
// the source's temporary unsynchronized mpq manager does.
func (m *HardwareFloatManager) ToRationalString(x *HardwareFloat) (string, error) {
	if m == nil || x == nil {
		return "", ErrNumericOwner
	}
	if modularSourceDebug && !(m.IsNormal(x) || m.IsDenormal(x) || m.IsZero(x)) {
		return "", ErrHardwareFloatNonRegular
	}
	sig := m.Significand(x)
	if m.IsNormal(x) {
		sig |= uint64(1) << 52
	}
	n := new(big.Int).SetUint64(sig)
	if m.Sign(x) {
		n.Neg(n)
	}
	d := new(big.Int).Lsh(big.NewInt(1), 52)
	exp := m.Exponent(x)
	if exp >= 0 {
		n.Lsh(n, uint(exp))
	} else {
		d.Lsh(d, uint(-exp))
	}
	return new(big.Rat).SetFrac(n, d).RatString(), nil
}

func (m *HardwareFloatNumeralManager) ToRationalString(x *HardwareFloat) (string, error) {
	if m == nil || m.core == nil {
		return "", ErrNumericOwner
	}
	return m.core.ToRationalString(x)
}
