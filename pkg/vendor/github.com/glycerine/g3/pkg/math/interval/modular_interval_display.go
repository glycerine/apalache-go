package interval

// Source: src/math/interval/mod_interval.h:232-239.

import "io"

func (v *ModularInterval) Display(w io.Writer) error {
	if v == nil || w == nil {
		return ErrExtendedDomain
	}
	if _, err := io.WriteString(w, "["); err != nil {
		return err
	}
	if _, err := io.WriteString(w, v.Lower().String()); err != nil {
		return err
	}
	if _, err := io.WriteString(w, ", "); err != nil {
		return err
	}
	if _, err := io.WriteString(w, v.Upper().String()); err != nil {
		return err
	}
	_, err := io.WriteString(w, "]")
	return err
}
