package numeral

// Source: src/util/mpf.cpp:1170-1181; src/util/mpf.h:236-238.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// ToMPZ truncates an unpacked MPF value using the source COPY, sign and shift
// sequence. The explicitly supplied Integer manager owns the output.
func (m *ArbitraryFloatManager) ToMPZ(x *ArbitraryFloat, output *IntegerManager, out *Int) (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	if m.toMPZ.active {
		return ErrNumericPending
	}
	if e := m.valueGuard(x); e != nil {
		return e
	}
	if output == nil || output.self != output {
		return umemory.ErrRuntimeOwner
	}
	if e := output.check(out); e != nil {
		return e
	}
	if e := mpfSourceAssertion(x.exponent < math.MaxInt32, 1172, "x.exponent < INT_MAX"); e != nil {
		return e
	}
	s := &m.toMPZ
	if s.invocation == ^uint64(0) {
		return ErrNumericPending
	}
	*s = arbitraryFloatToMPZScope{manager: m, output: output, invocation: s.invocation + 1, active: true, executing: true}
	s.hooks.Policy = s
	if e := s.bind(); e != nil {
		s.executing = false
		return e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		s.active = cause != nil || NumericAllocationPending(&s.numeric)
		cause = JoinNumericErrors(cause, s.unbind())
	}()
	return s.run(x, out)
}
func (s *arbitraryFloatToMPZScope) run(x *ArbitraryFloat, out *Int) error {
	if !s.NumericBound(true) {
		return ErrNumericOwner
	}
	z, c := s.output, s.context()
	if e := z.copyFromOwnerWithAllocationContext(out, s.manager.integer, &x.significand, &s.copy, c); e != nil {
		return e
	}
	if x.Sign() {
		if e := z.negWithAllocationContext(out, c); e != nil {
			return e
		}
	}
	// Unlike get_sbits(), the actual unsigned:16 bit-field promotes to int.
	// Guard the reached signed subtraction separately from the following +1.
	difference := int64(int32(x.exponent)) - int64(x.SBits())
	if difference < math.MinInt32 || difference > math.MaxInt32 {
		return arena.ErrBounds
	}
	if difference == math.MaxInt32 {
		return arena.ErrBounds
	}
	exponent := int32(difference + 1)
	if exponent < 0 {
		if exponent == math.MinInt32 {
			return arena.ErrBounds
		}
		return z.MachineDiv2kInPlace(out, uint32(-exponent))
	}
	if e := z.check(out); e != nil {
		return e
	}
	return z.mul2kCoreWithAllocationContext(out, uint32(exponent), c)
}
