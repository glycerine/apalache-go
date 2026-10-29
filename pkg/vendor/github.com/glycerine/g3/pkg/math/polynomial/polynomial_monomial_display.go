package polynomial

// Source: src/math/polynomial/polynomial.cpp; src/math/polynomial/polynomial.h.

import (
	"github.com/glycerine/g3/pkg/arena"
)

// MonomialTextSink is a borrowed cold output transport, never stored in a hot cell.
type MonomialTextSink interface {
	WriteLiteral(string) error
	WriteUnsigned(uint32) error
	WriteVariable(uint32) error
}

func monomialSinkLiteral(out MonomialTextSink, v string) error {
	if out == nil {
		return arena.ErrConfig
	}
	return out.WriteLiteral(v)
}
func monomialSinkVariable(out MonomialTextSink, v uint32) error {
	if out == nil {
		return arena.ErrConfig
	}
	return out.WriteVariable(v)
}
func monomialSinkUnsigned(out MonomialTextSink, v uint32) error {
	if out == nil {
		return arena.ErrConfig
	}
	return out.WriteUnsigned(v)
}
func (m *MonomialManager) DisplayInto(a Monomial, out MonomialTextSink, star bool) error {
	if e := m.check(); e != nil {
		return e
	}
	return m.displayIntoCore(a, out, star)
}
func (m *MonomialManager) displayIntoCore(a Monomial, out MonomialTextSink, star bool) error {
	n, e := m.sizeCore(a)
	if e != nil {
		return e
	}
	if n == 0 {
		return monomialSinkLiteral(out, "1")
	}
	for i := uint32(0); ; i++ {
		n, e = m.sizeCore(a)
		if e != nil {
			return e
		}
		if i >= n {
			break
		}
		if i > 0 {
			s := " "
			if star {
				s = "*"
			}
			if e = monomialSinkLiteral(out, s); e != nil {
				return e
			}
		}
		v, e := m.varCore(a, i)
		if e != nil {
			return e
		}
		if e = monomialSinkVariable(out, v); e != nil {
			return e
		}
		d, e := m.degreeAtCore(a, i)
		if e != nil {
			return e
		}
		if d > 1 {
			if e = monomialSinkLiteral(out, "^"); e != nil {
				return e
			}
			d, e = m.degreeAtCore(a, i)
			if e != nil {
				return e
			}
			if e = monomialSinkUnsigned(out, d); e != nil {
				return e
			}
		}
	}
	return nil
}
func (m *MonomialManager) DisplaySMT2Into(a Monomial, out MonomialTextSink) error {
	if e := m.check(); e != nil {
		return e
	}
	n, e := m.sizeCore(a)
	if e != nil {
		return e
	}
	if n == 0 {
		if e = monomialSinkLiteral(out, "1"); e != nil {
			return e
		}
	} else {
		single := false
		if n == 1 {
			d, e := m.degreeAtCore(a, 0)
			if e != nil {
				return e
			}
			single = d == 1
		}
		if single {
			v, e := m.varCore(a, 0)
			if e != nil {
				return e
			}
			if e = monomialSinkVariable(out, v); e != nil {
				return e
			}
		} else {
			if e = m.displaySMT2Product(a, out); e != nil {
				return e
			}
		}
	}
	return monomialSinkLiteral(out, ")")
}
func (m *MonomialManager) displaySMT2Product(a Monomial, out MonomialTextSink) error {
	if e := monomialSinkLiteral(out, "(*"); e != nil {
		return e
	}
	for i := uint32(0); ; i++ {
		n, e := m.sizeCore(a)
		if e != nil {
			return e
		}
		if i >= n {
			break
		}
		v, e := m.varCore(a, i)
		if e != nil {
			return e
		}
		d, e := m.degreeAtCore(a, i)
		if e != nil {
			return e
		}
		if e = m.sourceScalarAssert(408, "k > 0", d > 0); e != nil {
			return e
		}
		for j := uint32(0); j < d; j++ {
			if e = monomialSinkLiteral(out, " "); e != nil {
				return e
			}
			if e = monomialSinkVariable(out, v); e != nil {
				return e
			}
		}
	}
	return nil
}
