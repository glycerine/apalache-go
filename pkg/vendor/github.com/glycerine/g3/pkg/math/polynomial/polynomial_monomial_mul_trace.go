package polynomial

// Source: src/math/polynomial/polynomial.cpp:960-974; src/util/trace.h:40-46.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *MonomialManager) monomialTraceTags() *diagnostic.TraceTags {
	if m.config.Trace != nil {
		return m.config.Trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

func monomialTracePower(w io.Writer, p MonomialPower) error {
	if _, err := io.WriteString(w, "x"+strconv.FormatUint(uint64(p.Var), 10)); err != nil {
		return err
	}
	if p.Degree != 1 {
		_, err := io.WriteString(w, "^"+strconv.FormatUint(uint64(p.Degree), 10))
		return err
	}
	return nil
}

func monomialTracePowers(w io.Writer, label string, in monomialInput) error {
	if _, err := io.WriteString(w, label); err != nil {
		return err
	}
	for i := uint32(0); i < in.count; i++ {
		variable, err := in.variable(i)
		if err != nil {
			return err
		}
		if _, err = io.WriteString(w, "x"+strconv.FormatUint(uint64(variable), 10)); err != nil {
			return err
		}
		degree, err := in.degree(i)
		if err != nil {
			return err
		}
		if degree != 1 {
			degree, err = in.degree(i)
			if err != nil {
				return err
			}
			if _, err = io.WriteString(w, "^"+strconv.FormatUint(uint64(degree), 10)); err != nil {
				return err
			}
		}
		if _, err = io.WriteString(w, " "); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func (m *MonomialManager) monomialTraceTemp(w io.Writer, t *monomialTemp) error {
	if _, err := io.WriteString(w, "product_tmp: "); err != nil {
		return err
	}
	n, err := m.tempSize(t)
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		p, err := m.tempPower(t, i)
		if err != nil {
			return err
		}
		if err = monomialTracePower(w, p); err != nil {
			return err
		}
		if _, err = io.WriteString(w, " "); err != nil {
			return err
		}
	}
	_, err = io.WriteString(w, "\n")
	return err
}

func (m *MonomialManager) monomialTraceResult(w io.Writer, result Monomial) error {
	n, err := m.sizeCore(result)
	if err != nil {
		return err
	}
	if n == 0 {
		_, err = io.WriteString(w, "1")
		return err
	}
	for i := uint32(0); i < n; i++ {
		if i != 0 {
			if _, err = io.WriteString(w, " "); err != nil {
				return err
			}
		}
		variable, e := m.varCore(result, i)
		if e != nil {
			return e
		}
		if _, err = io.WriteString(w, "x"+strconv.FormatUint(uint64(variable), 10)); err != nil {
			return err
		}
		degree, e := m.degreeAtCore(result, i)
		if e != nil {
			return e
		}
		if degree > 1 {
			degree, e = m.degreeAtCore(result, i)
			if e != nil {
				return e
			}
			if _, err = io.WriteString(w, "^"+strconv.FormatUint(uint64(degree), 10)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *MonomialManager) traceMultiplyBefore(left, right monomialInput, t *monomialTemp) {
	tags := m.monomialTraceTags()
	if !tags.ShouldTrace("monomial_mul_bug") {
		return
	}
	_ = tags.EmitConditional("monomial_mul_bug", "mul", "src/math/polynomial/polynomial.cpp", 960,
		func() bool { return true }, func(w io.Writer) error {
			if _, err := io.WriteString(w, "before mk_monomial\n"); err != nil {
				return err
			}
			if err := monomialTracePowers(w, "pws1: ", left); err != nil {
				return err
			}
			if err := monomialTracePowers(w, "pws2: ", right); err != nil {
				return err
			}
			return m.monomialTraceTemp(w, t)
		})
}

func (m *MonomialManager) traceMultiplyAfter(left, right monomialInput, t *monomialTemp, count uint32, result Monomial) {
	tags := m.monomialTraceTags()
	if !tags.ShouldTrace("monomial_mul_bug") {
		return
	}
	_ = tags.EmitConditional("monomial_mul_bug", "mul", "src/math/polynomial/polynomial.cpp", 967,
		func() bool { return true }, func(w io.Writer) error {
			if _, err := io.WriteString(w, "j: "+strconv.FormatUint(uint64(count), 10)+"\nr: "); err != nil {
				return err
			}
			if err := m.monomialTraceResult(w, result); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
			if err := monomialTracePowers(w, "pws1: ", left); err != nil {
				return err
			}
			if err := monomialTracePowers(w, "pws2: ", right); err != nil {
				return err
			}
			return m.monomialTraceTemp(w, t)
		})
}
