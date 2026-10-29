package polynomial

// Source: src/math/polynomial/polynomial.cpp:2421; src/util/trace.h:40-46.

import (
	"io"
	"strconv"
)

// An ostream failure suppresses subsequent insertions without interrupting
// numerical formatting or its lexical cleanup.
type polynomialTraceSink struct {
	writer io.Writer
	failed bool
}

func (s *polynomialTraceSink) WriteLiteral(value string) error {
	if !s.failed {
		n, err := io.WriteString(s.writer, value)
		s.failed = err != nil || n != len(value)
	}
	return nil
}
func (s *polynomialTraceSink) Write(value []byte) (int, error) {
	_ = s.WriteLiteral(string(value))
	return len(value), nil
}
func (s *polynomialTraceSink) WriteUnsigned(value uint32) error {
	return s.WriteLiteral(strconv.FormatUint(uint64(value), 10))
}
func (s *polynomialTraceSink) WriteVariable(value uint32) error {
	return s.WriteLiteral("x" + strconv.FormatUint(uint64(value), 10))
}
func (s *polynomialTraceSink) WriteDecimal(value *PolynomialDecimalView) error {
	n, err := value.Length()
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		b, err := value.Byte(i)
		if err != nil {
			return err
		}
		if err = s.WriteLiteral(string(b)); err != nil {
			return err
		}
	}
	return nil
}

func (m *MultivariatePolynomialCore) traceDeletingPolynomial(p Polynomial) error {
	tags := m.monomials.monomialTraceTags()
	if !tags.ShouldTrace("polynomial") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("polynomial", "del", "src/math/polynomial/polynomial.cpp", 2421,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("deleting: ")
			formatErr = m.displayPolynomialDirect(p, sink, false)
			if formatErr != nil {
				return formatErr
			}
			return sink.WriteLiteral("\n")
		})
	return formatErr
}
