package polynomial

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Source: factorization.cpp:906-908,921-923,927-929,935-939. All
// polynomial insertions use the source integer core, including modular vectors.
func (s *UPolynomialFactorizationService) traceHenselVectors(caller *upolyHenselCallerScope, line int) error {
	tags := s.manager.modular.BorrowTraceTags()
	tag := "polynomial::factorization::bughunt"
	if line == 935 {
		tag = "polynomial::factorization"
	}
	if !tags.ShouldTrace(tag) {
		return nil
	}
	if caller == nil || !caller.matches() {
		return ErrUPolynomialOwner
	}
	c := caller.owner
	if e := c.traceBorrow.bindManagers([3]*UPolynomialManager{s.manager, caller.prime, caller.lift}, tags); e != nil {
		return e
	}
	defer c.traceBorrow.unbind()
	var computation error
	_ = tags.EmitConditional(tag, "hensel_lift", "src/math/polynomial/upolynomial_factorization.cpp", line,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			if line == 935 {
				_ = linearWriteText(out, "lifted to ")
				p, e := caller.lift.modular.BorrowedModulus()
				if e != nil {
					computation = e
					return e
				}
				prime, e := s.manager.modular.String(p)
				if e != nil {
					computation = e
					return e
				}
				_ = linearWriteText(out, prime)
				_ = diagnostic.WriteTraceEndl(out)
			}
			index, label := 0, "A = "
			if line == 921 {
				index, label = 2, "C = "
			} else if line == 927 {
				index, label = 1, "B = "
			}
			_ = linearWriteText(out, label)
			if e := s.manager.displayIntoWithReceipt(s.manager.stateView(&caller.outer[index]), factorizationTraceStream{out}, "x", false, &c.tracePolynomial); e != nil {
				computation = e
				return e
			}
			_ = diagnostic.WriteTraceEndl(out)
			if line == 935 {
				_ = linearWriteText(out, "B = ")
				if e := s.manager.displayIntoWithReceipt(s.manager.stateView(&caller.outer[1]), factorizationTraceStream{out}, "x", false, &c.tracePolynomial); e != nil {
					computation = e
					return e
				}
				_ = diagnostic.WriteTraceEndl(out)
			}
			return nil
		})
	return computation
}

// Source: factorization.cpp:886-888,960-962. Keep the source's "hansel"
// spelling. The exit record precedes the final inverse numeral's destructor.
func (s *UPolynomialFactorizationService) traceHenselFactors(caller *upolyHenselCallerScope, f UPolynomialView, pf, lf *UPolynomialFactors, exit bool) error {
	tags := s.manager.modular.BorrowTraceTags()
	if !tags.ShouldTrace("polynomial::factorization::bughunt") {
		return nil
	}
	if caller == nil || !caller.matches() {
		return ErrUPolynomialOwner
	}
	c := caller.owner
	if e := c.traceBorrow.bindManagers([3]*UPolynomialManager{s.manager, caller.prime, caller.lift}, tags); e != nil {
		return e
	}
	defer c.traceBorrow.unbind()
	line := 886
	if exit {
		line = 960
	}
	var computation error
	_ = tags.EmitConditional("polynomial::factorization::bughunt", "hensel_lift", "src/math/polynomial/upolynomial_factorization.cpp", line,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			_ = linearWriteText(out, "polynomial::hansel_lift(")
			if e := s.manager.displayIntoWithReceipt(f, factorizationTraceStream{out}, "x", false, &c.tracePolynomial); e != nil {
				computation = e
				return e
			}
			_ = linearWriteText(out, ", ")
			if e := pf.displayIntoWithReceipt(factorizationTraceStream{out}, &c.trace); e != nil {
				computation = e
				return e
			}
			if exit {
				_ = linearWriteText(out, ") => ")
				if e := lf.displayIntoWithReceipt(factorizationTraceStream{out}, &c.trace); e != nil {
					computation = e
					return e
				}
			} else {
				_ = linearWriteText(out, ")")
			}
			return diagnostic.WriteTraceEndl(out)
		})
	return computation
}

// Source: factorization.cpp:744-748 and771-774. These displays borrow
// the actual quadratic related core as well as root/prime/lift owners.
func (s *UPolynomialFactorizationService) traceHenselQuadratic(caller *upolyHenselCallerScope, C UPolynomialView, pe *unumeral.Int, line int) error {
	tags := s.manager.modular.BorrowTraceTags()
	if !tags.ShouldTrace("polynomial::factorization::bughunt") {
		return nil
	}
	if caller == nil || !caller.matches() {
		return ErrUPolynomialOwner
	}
	c := caller.owner
	if e := c.traceBorrow.bindFamily([4]*UPolynomialManager{s.manager, caller.prime, caller.lift, caller.related}, tags); e != nil {
		return e
	}
	defer c.traceBorrow.unbind()
	var computation error
	_ = tags.EmitConditional("polynomial::factorization::bughunt", "hensel_lift_quadratic", "src/math/polynomial/upolynomial_factorization.cpp", line,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			if line == 771 {
				_ = linearWriteText(out, "g = (1 - A_lifted*U - B_lifted*V)/")
				prime, e := s.manager.config.Integer.String(pe)
				if e != nil {
					computation = e
					return e
				}
				_ = linearWriteText(out, prime)
				_ = diagnostic.WriteTraceEndl(out)
				_ = linearWriteText(out, "g == ")
				if e := s.manager.displayIntoWithReceipt(s.manager.stateView(&caller.iteration[1]), factorizationTraceStream{out}, "x", false, &c.tracePolynomial); e != nil {
					computation = e
					return e
				}
				_ = diagnostic.WriteTraceEndl(out)
			} else {
				views := [3]UPolynomialView{s.manager.stateView(&caller.quadratic[3]), s.manager.stateView(&caller.quadratic[4]), C}
				for i, label := range [3]string{"A_lifted = ", "B_lifted = ", "C = "} {
					_ = linearWriteText(out, label)
					if e := s.manager.displayIntoWithReceipt(views[i], factorizationTraceStream{out}, "x", false, &c.tracePolynomial); e != nil {
						computation = e
						return e
					}
					_ = diagnostic.WriteTraceEndl(out)
				}
			}
			return nil
		})
	return computation
}

// Source: factorization.cpp:713-719, before the quadratic related core exists.
func (s *UPolynomialFactorizationService) traceHenselQuadraticEntry(caller *upolyHenselCallerScope, zpe *UPolynomialManager, A, B, C UPolynomialView, exponent uint32) error {
	tags := s.manager.modular.BorrowTraceTags()
	if !tags.ShouldTrace("polynomial::factorization::bughunt") {
		return nil
	}
	if caller == nil || !caller.matches() {
		return ErrUPolynomialOwner
	}
	c := caller.owner
	if e := c.traceBorrow.bindFamily([4]*UPolynomialManager{s.manager, caller.prime, zpe, caller.related}, tags); e != nil {
		return e
	}
	defer c.traceBorrow.unbind()
	var computation error
	_ = tags.EmitConditional("polynomial::factorization::bughunt", "hensel_lift_quadratic", "src/math/polynomial/upolynomial_factorization.cpp", 713,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			_ = linearWriteText(out, "polynomial::hansel_lift_quadratic(")
			for i, v := range [3]UPolynomialView{A, B, C} {
				_ = linearWriteText(out, [3]string{"A = ", "B = ", "C = "}[i])
				if e := s.manager.displayIntoWithReceipt(v, factorizationTraceStream{out}, "x", false, &c.tracePolynomial); e != nil {
					computation = e
					return e
				}
				_ = linearWriteText(out, ", ")
			}
			_ = linearWriteText(out, "p = ")
			p, e := zpe.modular.BorrowedModulus()
			if e != nil {
				computation = e
				return e
			}
			prime, e := s.manager.config.Integer.String(p)
			if e != nil {
				computation = e
				return e
			}
			_ = linearWriteText(out, prime)
			_ = linearWriteText(out, ", e = ")
			_ = linearWriteText(out, strconv.FormatUint(uint64(exponent), 10))
			_ = linearWriteText(out, ")")
			return diagnostic.WriteTraceEndl(out)
		})
	return computation
}

// Source: factorization.cpp:688-698, reached only when UA+VB is not one.
// This diagnostic uses the lifted modular display core, as in the source.
func (s *UPolynomialFactorizationService) traceQuadraticCheckFailure(caller *upolyHenselCallerScope, zpe *UPolynomialManager, U, A, V, B UPolynomialView) error {
	tags := s.manager.modular.BorrowTraceTags()
	if !tags.ShouldTrace("polynomial::factorization::bughunt") {
		return nil
	}
	if caller == nil || !caller.matches() {
		return ErrUPolynomialOwner
	}
	c := caller.owner
	if e := c.traceBorrow.bindFamily([4]*UPolynomialManager{s.manager, caller.prime, zpe, caller.related}, tags); e != nil {
		return e
	}
	defer c.traceBorrow.unbind()
	var computation error
	_ = tags.EmitConditional("polynomial::factorization::bughunt", "check_quadratic_hensel", "src/math/polynomial/upolynomial_factorization.cpp", 688,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			_ = linearWriteText(out, "sage: R.<x> = Zmod(")
			p, e := zpe.modular.BorrowedModulus()
			if e != nil {
				computation = e
				return e
			}
			prime, e := s.manager.config.Integer.String(p)
			if e != nil {
				computation = e
				return e
			}
			_ = linearWriteText(out, prime)
			_ = linearWriteText(out, ")['x']")
			_ = diagnostic.WriteTraceEndl(out)
			for i, v := range [4]UPolynomialView{A, B, U, V} {
				_ = linearWriteText(out, [4]string{"sage: A = ", "sage: B = ", "sage: U = ", "sage: V = "}[i])
				if e := zpe.displayIntoWithReceipt(v, factorizationTraceStream{out}, "x", false, &c.tracePolynomial); e != nil {
					computation = e
					return e
				}
				_ = diagnostic.WriteTraceEndl(out)
			}
			_ = linearWriteText(out, "sage: print  (1 - UA - VB)")
			_ = diagnostic.WriteTraceEndl(out)
			_ = linearWriteText(out, "sage: print 'expected 0'")
			return diagnostic.WriteTraceEndl(out)
		})
	return computation
}

// Source: factorization.cpp:826-833 and848-855, only on unequal products.
func (s *UPolynomialFactorizationService) traceHenselProductFailure(caller *upolyHenselCallerScope, f UPolynomialView, factors *UPolynomialFactors, prime *unumeral.Int, lifted bool) error {
	tags := s.manager.modular.BorrowTraceTags()
	if !tags.ShouldTrace("polynomial::factorization::bughunt") {
		return nil
	}
	if caller == nil || !caller.matches() {
		return ErrUPolynomialOwner
	}
	c := caller.owner
	if e := c.traceBorrow.bindFamily([4]*UPolynomialManager{s.manager, caller.prime, caller.lift, caller.related}, tags); e != nil {
		return e
	}
	defer c.traceBorrow.unbind()
	line, index, suffix := 826, 0, "zp"
	if lifted {
		line, index, suffix = 848, 2, "zpe"
	}
	var computation error
	_ = tags.EmitConditional("polynomial::factorization::bughunt", "check_hensel_lift", "src/math/polynomial/upolynomial_factorization.cpp", line,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			_ = linearWriteText(out, "f = ")
			if e := s.manager.displayIntoWithReceipt(f, factorizationTraceStream{out}, "x", false, &c.tracePolynomial); e != nil {
				computation = e
				return e
			}
			_ = diagnostic.WriteTraceEndl(out)
			_ = linearWriteText(out, suffix+"_fs = ")
			if e := factors.displayIntoWithReceipt(factorizationTraceStream{out}, &c.trace); e != nil {
				computation = e
				return e
			}
			_ = diagnostic.WriteTraceEndl(out)
			_ = linearWriteText(out, "sage: R.<x> = Zmod(")
			p, e := s.manager.modular.String(prime)
			if e != nil {
				computation = e
				return e
			}
			_ = linearWriteText(out, p)
			_ = linearWriteText(out, ")['x']")
			_ = diagnostic.WriteTraceEndl(out)
			for i, label := range [2]string{"sage: mult_" + suffix + " = ", "sage: f_" + suffix + " = "} {
				_ = linearWriteText(out, label)
				if e := s.manager.displayIntoWithReceipt(s.manager.stateView(&caller.checkVectors[index+i]), factorizationTraceStream{out}, "x", false, &c.tracePolynomial); e != nil {
					computation = e
					return e
				}
				_ = diagnostic.WriteTraceEndl(out)
			}
			_ = linearWriteText(out, "sage: mult_"+suffix+" == f_"+suffix)
			return diagnostic.WriteTraceEndl(out)
		})
	return computation
}
