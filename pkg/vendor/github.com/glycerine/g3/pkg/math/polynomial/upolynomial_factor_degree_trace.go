package polynomial

// Source: upolynomial_factorization.cpp:1141-1145,1155-1158,1192-1196.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// The diagnostic writer retains ostream failure. Suppress insertion errors
// while preserving later operand computation and the reached endl flushes.
type factorizationTraceWriter struct{ io.Writer }

func (w factorizationTraceWriter) Write(p []byte) (int, error) {
	_, _ = w.Writer.Write(p)
	return len(p), nil
}
func (w factorizationTraceWriter) Flush() error {
	if f, ok := w.Writer.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}
	return nil
}

type factorizationTraceStream struct{ writer io.Writer }

func (s factorizationTraceStream) AppendCString(v string) error { return linearWriteText(s.writer, v) }
func (s factorizationTraceStream) AppendNumeralText(v string) error {
	return linearWriteText(s.writer, v)
}
func (s factorizationTraceStream) AppendInt32(v int32) error {
	return linearWriteText(s.writer, strconv.FormatInt(int64(v), 10))
}

func (s *UPolynomialFactorizationService) traceFactorDegreeSet(fs *UPolynomialFactors, line int, trial bool) error {
	tags := s.manager.modular.BorrowTraceTags()
	if !tags.ShouldTrace("polynomial::factorization::bughunt") {
		return nil
	}
	c := s.scope
	if c == nil || c.self != c {
		return ErrUPolynomialOwner
	}
	if e := c.traceBorrow.bind(c, tags); e != nil {
		return e
	}
	defer c.traceBorrow.unbind()
	var computation error
	_ = tags.EmitConditional("polynomial::factorization::bughunt", "factor_square_free", "src/math/polynomial/upolynomial_factorization.cpp", line,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			if trial {
				_ = linearWriteText(out, "STARTING TRIAL DIVISION")
				_ = diagnostic.WriteTraceEndl(out)
				_ = linearWriteText(out, "zpe_fs")
			} else {
				_ = linearWriteText(out, "best zp factorization (Z_")
				prime, e := s.manager.modular.String(&c.bestP)
				if e != nil {
					computation = e
					return e
				}
				_ = linearWriteText(out, prime)
				_ = linearWriteText(out, "): ")
			}
			if e := fs.displayIntoWithReceipt(factorizationTraceStream{out}, &c.trace); e != nil {
				computation = e
				return e
			}
			_ = diagnostic.WriteTraceEndl(out)
			label := "best degree set: "
			if trial {
				label = "degree_set = "
			}
			_ = linearWriteText(out, label)
			if e := c.ds.display(out); e != nil {
				computation = e
				return e
			}
			return diagnostic.WriteTraceEndl(out)
		})
	return computation
}

// Source: upolynomial_factorization.cpp:1088-1091. Each to_string is
// evaluated at its insertion point, including after an ostream failure.
func (s *UPolynomialFactorizationService) traceFactorPrimeGCD(top *unumeral.Int) error {
	tags := s.manager.modular.BorrowTraceTags()
	if !tags.ShouldTrace("polynomial::factorization::bughunt") {
		return nil
	}
	c := s.scope
	if c == nil || c.self != c {
		return ErrUPolynomialOwner
	}
	if e := c.traceBorrow.bind(c, tags); e != nil {
		return e
	}
	defer c.traceBorrow.unbind()
	var computation error
	_ = tags.EmitConditional("polynomial::factorization::bughunt", "factor_square_free", "src/math/polynomial/upolynomial_factorization.cpp", 1088,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			_ = linearWriteText(out, "sage: if (not (gcd(")
			p, e := s.manager.modular.String(&c.p)
			if e != nil {
				computation = e
				return e
			}
			_ = linearWriteText(out, p)
			_ = linearWriteText(out, ", ")
			leading, e := s.manager.modular.String(top)
			if e != nil {
				computation = e
				return e
			}
			_ = linearWriteText(out, leading)
			_ = linearWriteText(out, ")) == ")
			gcd, e := s.manager.modular.String(&c.gcdTmp)
			if e != nil {
				computation = e
				return e
			}
			_ = linearWriteText(out, gcd)
			_ = linearWriteText(out, "): print 'Error, wrong gcd'")
			return diagnostic.WriteTraceEndl(out)
		})
	return computation
}

// Source: upolynomial_factorization.cpp:1163-1165, before lift-manager construction.
func (s *UPolynomialFactorizationService) traceFactorLiftExponent(exponent uint32) error {
	tags := s.manager.modular.BorrowTraceTags()
	if !tags.ShouldTrace("polynomial::factorization::bughunt") {
		return nil
	}
	c := s.scope
	if c == nil || c.self != c {
		return ErrUPolynomialOwner
	}
	if e := c.traceBorrow.bind(c, tags); e != nil {
		return e
	}
	defer c.traceBorrow.unbind()
	var computation error
	_ = tags.EmitConditional("polynomial::factorization::bughunt", "factor_square_free", "src/math/polynomial/upolynomial_factorization.cpp", 1163,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			_ = linearWriteText(out, "out p = ")
			prime, e := s.manager.modular.String(&c.bestP)
			if e != nil {
				computation = e
				return e
			}
			_ = linearWriteText(out, prime)
			_ = linearWriteText(out, ", and we'll work p^e for e = ")
			_ = linearWriteText(out, strconv.FormatUint(uint64(exponent), 10))
			return diagnostic.WriteTraceEndl(out)
		})
	return computation
}
