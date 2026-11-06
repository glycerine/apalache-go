package interval

// Source: src/math/interval/interval_def.h:1019-1218, 1271-1378,
// 1510-1590, and 1663-1849.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (v *RationalInterval) intervalTraceTags() *diagnostic.TraceTags {
	if v.TraceTags != nil {
		return v.TraceTags
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (v *RationalInterval) traceIntervalBug(function string, line int, body func(io.Writer) error) {
	tags := v.intervalTraceTags()
	if !tags.ShouldTrace("interval_bug") {
		return
	}
	_ = tags.EmitConditional("interval_bug", function, "src/math/interval/interval_def.h", line,
		func() bool { return true }, body)
}

func (v *RationalInterval) traceIntervalValue(w io.Writer, value *RationalInterval) error {
	return value.Display(w)
}

func (v *RationalInterval) traceIntervalNumeral(w io.Writer, value *numeral.Rational) error {
	return v.numbers.DisplayTo(w, value)
}

func traceIntervalPart(w io.Writer, s string) { _, _ = io.WriteString(w, s) }

func traceIntervalCallID(w io.Writer, id uint64) {
	traceIntervalPart(w, strconv.FormatUint(id, 10))
}

func (v *RationalInterval) traceMulCase(id uint64, branch string, line int, a, b *RationalInterval, detail bool) {
	v.traceIntervalBug("mul", line, func(w io.Writer) error {
		traceIntervalPart(w, branch+" #")
		traceIntervalCallID(w, id)
		traceIntervalPart(w, "\n")
		if err := v.traceIntervalValue(w, a); err != nil {
			return err
		}
		traceIntervalPart(w, "\n")
		if err := v.traceIntervalValue(w, b); err != nil {
			return err
		}
		traceIntervalPart(w, "\n")
		if !detail {
			return nil
		}
		al, au := a.endpoints()
		bl, bu := b.endpoints()
		for _, item := range [...]struct {
			name string
			v    *numeral.Rational
		}{{"a", al.value}, {"b", au.value}, {"c", bl.value}, {"d", bu.value}} {
			traceIntervalPart(w, item.name+": ")
			if err := v.traceIntervalNumeral(w, item.v); err != nil {
				return err
			}
			traceIntervalPart(w, "\n")
		}
		az, err := a.closedZeroAtUpper()
		if err != nil {
			return err
		}
		bz, err := b.closedZeroAtUpper()
		if err != nil {
			return err
		}
		traceIntervalPart(w, "is_N0(i1): "+strconv.FormatBool(az)+"\nis_N0(i2): "+strconv.FormatBool(bz)+"\n")
		return nil
	})
}

func (v *RationalInterval) traceBinaryIntervalCall(function string, line int, name string, id uint64, a, b *RationalInterval, detail bool) {
	v.traceIntervalBug(function, line, func(w io.Writer) error {
		traceIntervalPart(w, name+" #")
		traceIntervalCallID(w, id)
		traceIntervalPart(w, "\n")
		if err := v.traceIntervalValue(w, a); err != nil {
			return err
		}
		traceIntervalPart(w, "\n")
		if err := v.traceIntervalValue(w, b); err != nil {
			return err
		}
		traceIntervalPart(w, "\n")
		if detail {
			al, au := a.endpoints()
			bl, bu := b.endpoints()
			for _, item := range [...]struct {
				name  string
				value *numeral.Rational
			}{{"a", al.value}, {"b", au.value}, {"c", bl.value}, {"d", bu.value}} {
				traceIntervalPart(w, item.name+": ")
				if err := v.traceIntervalNumeral(w, item.value); err != nil {
					return err
				}
				traceIntervalPart(w, "\n")
			}
		}
		return nil
	})
}

func (v *RationalInterval) traceUnaryIntervalCall(function string, line int, name string, id uint64, a *RationalInterval, degree uint32) {
	v.traceIntervalBug(function, line, func(w io.Writer) error {
		traceIntervalPart(w, name+" #")
		traceIntervalCallID(w, id)
		traceIntervalPart(w, "\n")
		if err := v.traceIntervalValue(w, a); err != nil {
			return err
		}
		traceIntervalPart(w, "\n")
		if name == "(M)" {
			traceIntervalPart(w, "n:"+strconv.FormatUint(uint64(degree), 10)+"\n")
		}
		return nil
	})
}

func (v *RationalInterval) traceIntervalResult(function string, line int, result *RationalInterval) {
	v.traceIntervalBug(function, line, func(w io.Writer) error {
		traceIntervalPart(w, "result: ")
		if err := v.traceIntervalValue(w, result); err != nil {
			return err
		}
		traceIntervalPart(w, "\n")
		return nil
	})
}

func (v *RationalInterval) traceIntervalBranch(function string, line int, branch string, id uint64) {
	v.traceIntervalBug(function, line, func(w io.Writer) error {
		traceIntervalPart(w, branch+" #")
		traceIntervalCallID(w, id)
		traceIntervalPart(w, "\n")
		return nil
	})
}

func (v *RationalInterval) traceIntervalKinds(function string, line int, lower, upper ExtendedKind) {
	v.traceIntervalBug(function, line, func(w io.Writer) error {
		traceIntervalPart(w, "new_l_kind: "+strconv.FormatUint(uint64(lower), 10)+", new_u_kind: "+strconv.FormatUint(uint64(upper), 10)+"\n")
		return nil
	})
}
