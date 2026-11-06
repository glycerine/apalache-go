package numeral

// Source: src/util/f2n.h:25-101, instantiated as f2n<mpf_manager> by
// src/math/subpaving/tactic/subpaving_tactic.cpp:58.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

var ErrArbitraryFloatNonRegular = errors.New("numeral: non-regular arbitrary float")
var ErrArbitraryFloatNumeralRawPending = errors.New("numeral: arbitrary-float numeral raw local cleanup pending")

// ArbitraryFloatNumeralManager is the concrete subpaving f2n<mpf_manager>
// wrapper. It borrows core and owns its source scratch numerals.
type ArbitraryFloatNumeralManager struct {
	core         *ArbitraryFloatManager
	mode         ArbitraryFloatRoundingMode
	ebits, sbits uint32
	tmp, one     ArbitraryFloat
	rawLocal     ArbitraryFloat
	rawPending   bool
	closed       bool
}

func NewArbitraryFloatNumeralManager(core *ArbitraryFloatManager, ebits, sbits uint32) (*ArbitraryFloatNumeralManager, error) {
	if core == nil {
		return nil, ErrNumericOwner
	}
	if e := core.valueOwnerGuard(); e != nil {
		return nil, e
	}
	m := &ArbitraryFloatNumeralManager{core: core, mode: ArbitraryFloatTowardPositive, ebits: ebits, sbits: sbits}
	if e := core.SetInt32(&m.one, ebits, sbits, 1); e != nil {
		return nil, errors.Join(e, core.CleanupValue(&m.one))
	}
	return m, nil
}

func (m *ArbitraryFloatNumeralManager) guard() error {
	if m == nil || m.core == nil {
		return ErrNumericOwner
	}
	if m.closed {
		return arena.ErrClosed
	}
	return m.core.valueOwnerGuard()
}

func (m *ArbitraryFloatNumeralManager) check(out *ArbitraryFloat) error {
	if e := m.guard(); e != nil {
		return e
	}
	regular, e := m.core.IsRegular(out)
	if e != nil {
		return e
	}
	if !regular {
		return ErrArbitraryFloatNonRegular
	}
	return nil
}

// Close follows the source destructor's scratch order and leaves core owned
// by its caller. A failed deletion can be retried after its cause is resolved.
func (m *ArbitraryFloatNumeralManager) Close() error {
	if m == nil || m.core == nil {
		return ErrNumericOwner
	}
	if m.closed {
		return nil
	}
	if e := m.CleanupRawLocal(); e != nil {
		return e
	}
	if e := m.core.Del(&m.tmp); e != nil {
		return e
	}
	if e := m.core.Del(&m.one); e != nil {
		return e
	}
	m.closed = true
	return nil
}

func (m *ArbitraryFloatNumeralManager) RoundingMode() ArbitraryFloatRoundingMode { return m.mode }
func (m *ArbitraryFloatNumeralManager) ExponentBits() uint32                     { return m.ebits }
func (m *ArbitraryFloatNumeralManager) SignificandBits() uint32                  { return m.sbits }

// BorrowCore is the concrete f2n<mpf_manager>::m() reference. The returned
// manager belongs to the caller and must outlive this numeral wrapper.
func (m *ArbitraryFloatNumeralManager) BorrowCore() (*ArbitraryFloatManager, error) {
	if err := m.guard(); err != nil {
		return nil, err
	}
	return m.core, nil
}

func (m *ArbitraryFloatNumeralManager) RoundToPlusInf()  { m.mode = ArbitraryFloatTowardPositive }
func (m *ArbitraryFloatNumeralManager) RoundToMinusInf() { m.mode = ArbitraryFloatTowardNegative }
func (m *ArbitraryFloatNumeralManager) SetRounding(toPlus bool) {
	if toPlus {
		m.RoundToPlusInf()
	} else {
		m.RoundToMinusInf()
	}
}

func (m *ArbitraryFloatNumeralManager) SetInt32(out *ArbitraryFloat, value int32) error {
	if e := m.guard(); e != nil {
		return e
	}
	if e := m.core.SetInt32(out, m.ebits, m.sbits, value); e != nil {
		return e
	}
	return m.check(out)
}
func (m *ArbitraryFloatNumeralManager) SetDouble(out *ArbitraryFloat, value float64) error {
	if e := m.guard(); e != nil {
		return e
	}
	if e := m.core.SetDouble(out, m.ebits, m.sbits, value); e != nil {
		return e
	}
	return m.check(out)
}
func (m *ArbitraryFloatNumeralManager) SetFractionSigned(out *ArbitraryFloat, n, d int32) error {
	if e := m.guard(); e != nil {
		return e
	}
	if e := m.core.SetFractionSigned(out, m.ebits, m.sbits, m.mode, n, d); e != nil {
		return e
	}
	return m.check(out)
}
func (m *ArbitraryFloatNumeralManager) SetRational(out *ArbitraryFloat, value *Rational) error {
	if e := m.guard(); e != nil {
		return e
	}
	if e := m.core.SetRational(out, m.ebits, m.sbits, m.mode, value); e != nil {
		return e
	}
	return m.check(out)
}
func (m *ArbitraryFloatNumeralManager) Copy(out, in *ArbitraryFloat) error {
	if e := m.guard(); e != nil {
		return e
	}
	if e := m.core.Copy(out, in); e != nil {
		return e
	}
	return m.check(out)
}
func (m *ArbitraryFloatNumeralManager) Reset(out *ArbitraryFloat) error {
	if e := m.guard(); e != nil {
		return e
	}
	return m.core.Reset(out, m.ebits, m.sbits)
}

func (m *ArbitraryFloatNumeralManager) Add(x, y, out *ArbitraryFloat) error {
	if e := m.guard(); e != nil {
		return e
	}
	if e := m.core.Add(m.mode, x, y, out); e != nil {
		return e
	}
	return m.check(out)
}
func (m *ArbitraryFloatNumeralManager) Sub(x, y, out *ArbitraryFloat) error {
	if e := m.guard(); e != nil {
		return e
	}
	if e := m.core.Sub(m.mode, x, y, out); e != nil {
		return e
	}
	return m.check(out)
}
func (m *ArbitraryFloatNumeralManager) Mul(x, y, out *ArbitraryFloat) error {
	if e := m.guard(); e != nil {
		return e
	}
	if e := m.core.Mul(m.mode, x, y, out); e != nil {
		return e
	}
	return m.check(out)
}
func (m *ArbitraryFloatNumeralManager) Div(x, y, out *ArbitraryFloat) error {
	if e := m.guard(); e != nil {
		return e
	}
	if e := m.core.Div(m.mode, x, y, out); e != nil {
		return e
	}
	return m.check(out)
}
