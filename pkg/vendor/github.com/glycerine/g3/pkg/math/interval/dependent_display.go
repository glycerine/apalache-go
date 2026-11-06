package interval

// Source: src/math/interval/dep_intervals.cpp:89-117.

import (
	"io"
	"strconv"
)

func (v *DependentRationalInterval) Display(w io.Writer) error {
	if err := v.open(); err != nil {
		return err
	}
	if v.Value.lowerInf {
		if err := writeExtendedText(w, "(-oo"); err != nil {
			return err
		}
	} else {
		left := "["
		if v.Value.lowerOpen {
			left = "("
		}
		if err := writeExtendedText(w, left); err != nil {
			return err
		}
		if err := v.manager.Numbers.DisplayTo(w, &v.Value.lower); err != nil {
			return err
		}
	}
	if err := writeExtendedText(w, ","); err != nil {
		return err
	}
	if v.Value.upperInf {
		if err := writeExtendedText(w, "oo)"); err != nil {
			return err
		}
	} else {
		if err := v.manager.Numbers.DisplayTo(w, &v.Value.upper); err != nil {
			return err
		}
		right := "]"
		if v.Value.upperOpen {
			right = ")"
		}
		if err := writeExtendedText(w, right); err != nil {
			return err
		}
	}
	for _, item := range []struct {
		label  string
		values []uint32
	}{{" ld", v.LowerDependencies()}, {" ud", v.UpperDependencies()}} {
		if len(item.values) == 0 {
			continue
		}
		if _, err := io.WriteString(w, item.label); err != nil {
			return err
		}
		for _, dependency := range item.values {
			if _, err := io.WriteString(w, " "+strconv.FormatUint(uint64(dependency), 10)); err != nil {
				return err
			}
		}
	}
	return nil
}
