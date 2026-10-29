package numeral

// Source: src/util/mpff.cpp:1173-1224, retained subpaving interval display.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
)

func writeFastFloatPart(w io.Writer, s string) error {
	n, err := io.WriteString(w, s)
	if err == nil && n != len(s) {
		return io.ErrShortWrite
	}
	return err
}

// DisplayTo follows the source's sign, significand and exponent insertions.
// A refused insertion leaves the completed prefix in the caller's writer.
func (m *FastFloatManager) DisplayTo(w io.Writer, a *FastFloat) (err error) {
	if w == nil {
		return arena.ErrInvalidSlot
	}
	if err = m.check(a); err != nil {
		return err
	}
	if a.negative() {
		if err = writeFastFloatPart(w, "-"); err != nil {
			return err
		}
	}
	m.toBuffer(0, a, true, false)
	v := m.buffer(0, m.precision)
	tz := ffNTZ(v)
	exp := int64(a.exponent)
	shift := uint32(0)
	if exp < 0 {
		if int64(tz) >= -exp {
			shift = uint32(-exp)
			exp = 0
		} else {
			shift = tz
			exp += int64(tz)
		}
	}
	if shift != 0 {
		ffShift(v, v, shift, false)
	}
	if err = m.beginFastFormatting(); err != nil {
		return err
	}
	defer func() { err = m.endFastFormatting(err) }()
	n, e := m.beginFastCharacters(v, 0)
	if e != nil {
		return e
	}
	digits, e := m.natural.finalString(n)
	if e != nil {
		return e
	}
	if e = writeFastFloatPart(w, digits); e != nil {
		return e
	}
	if exp == 0 {
		return nil
	}
	mag := uint64(exp)
	if exp < 0 {
		mag = uint64(-exp)
	}
	if mag <= 63 {
		operator := "*"
		if exp < 0 {
			operator = "/"
		}
		if e = writeFastFloatPart(w, operator); e != nil {
			return e
		}
		return writeFastFloatPart(w, strconv.FormatUint(uint64(1)<<uint(mag), 10))
	}
	operator := "*2"
	if exp < 0 {
		operator = "/2"
	}
	if e = writeFastFloatPart(w, operator); e != nil {
		return e
	}
	if e = writeFastFloatPart(w, "^"); e != nil {
		return e
	}
	return writeFastFloatPart(w, strconv.FormatUint(mag, 10))
}
