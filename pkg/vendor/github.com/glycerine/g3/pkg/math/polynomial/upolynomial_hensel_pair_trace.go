package polynomial

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (h *upolyHenselPairScope) traceSourceBlocker() error {
	m := h.service.manager
	if !m.displayReceiptPending(&h.trace) {
		return nil
	}
	if e := m.displayReceiptSourceCause(&h.trace); e != nil {
		return e
	}
	return ErrUPolynomialCleanupPending
}

// Complete intermediate records in the concrete two-factor Hensel algorithm.
// Polynomial insertion always uses the integer core, as in the C++ source.
func (s *UPolynomialFactorizationService) traceHenselPairStage(h *upolyHenselPairScope, line int) error {
	tag := "polynomial::factorization::bughunt"
	if line == 614 {
		tag = "polynomial::factorization"
	}
	tags := s.manager.modular.BorrowTraceTags()
	if !tags.ShouldTrace(tag) {
		return nil
	}
	if h == nil || h.self != h || h.service != s || !h.active {
		return ErrUPolynomialOwner
	}
	if e := h.traceBorrow.bindManagers([3]*UPolynomialManager{s.manager, h.rm}, tags); e != nil {
		return e
	}
	defer h.traceBorrow.unbind()
	var computation error
	_ = tags.EmitConditional(tag, "hensel_lift", "src/math/polynomial/upolynomial_factorization.cpp", line,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			label, caption := "f = ", ""
			view := s.manager.stateView(&h.f)
			switch line {
			case 637:
				label = "V == "
				view = h.borrowedInput(2)
			case 641:
				caption = "Vf = V*f"
				label = "Vf == "
				view = s.manager.stateView(&h.Vf)
			case 646:
				caption = "[t, S] = Vf.quo_rem(A)"
				label = "t == "
				view = s.manager.stateView(&h.t)
			case 653:
				caption = "T == U*f"
				label = "T == "
				view = s.manager.stateView(&h.T)
			case 658:
				caption = "tmp = B*t"
				label = "tmp == "
				view = s.manager.stateView(&h.tmp)
			case 663:
				caption = "T = B*tmp"
				label = "T == "
				view = s.manager.stateView(&h.T)
			}
			if caption != "" {
				_ = linearWriteText(out, caption)
				_ = diagnostic.WriteTraceEndl(out)
			}
			_ = linearWriteText(out, label)
			if e := s.manager.displayIntoWithReceipt(view, factorizationTraceStream{out}, "x", false, &h.trace); e != nil {
				computation = e
				return e
			}
			_ = diagnostic.WriteTraceEndl(out)
			if line == 646 {
				_ = linearWriteText(out, "S == ")
				if e := s.manager.displayIntoWithReceipt(s.manager.stateView(&h.S), factorizationTraceStream{out}, "x", false, &h.trace); e != nil {
					computation = e
					return e
				}
				_ = diagnostic.WriteTraceEndl(out)
			}
			return nil
		})
	return computation
}

// Source: factorization.cpp:590-600, before constructing the related core.
func (s *UPolynomialFactorizationService) traceHenselPairEntry(h *upolyHenselPairScope, a, b, r *unumeral.Int) error {
	tags := s.manager.modular.BorrowTraceTags()
	if !tags.ShouldTrace("polynomial::factorization::bughunt") {
		return nil
	}
	if h == nil || h.self != h || h.service != s || !h.active {
		return ErrUPolynomialOwner
	}
	if e := h.traceBorrow.bindManagers([3]*UPolynomialManager{s.manager}, tags); e != nil {
		return e
	}
	defer h.traceBorrow.unbind()
	var computation error
	_ = tags.EmitConditional("polynomial::factorization::bughunt", "hensel_lift", "src/math/polynomial/upolynomial_factorization.cpp", 590,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			_ = linearWriteText(out, "polynomial::hensel_lift(")
			for i, v := range [3]*unumeral.Int{a, b, r} {
				_ = linearWriteText(out, [3]string{"a = ", "b = ", "r = "}[i])
				text, e := s.manager.config.Integer.String(v)
				if e != nil {
					computation = e
					return e
				}
				_ = linearWriteText(out, text)
				_ = linearWriteText(out, ", ")
			}
			for i, label := range [5]string{"U = ", "A = ", "V = ", "B = ", "C = "} {
				_ = linearWriteText(out, label)
				if e := s.manager.displayIntoWithReceipt(h.borrowedInput(i), factorizationTraceStream{out}, "x", false, &h.trace); e != nil {
					computation = e
					return e
				}
				if i != 4 {
					_ = linearWriteText(out, ", ")
				}
			}
			_ = linearWriteText(out, ")")
			return diagnostic.WriteTraceEndl(out)
		})
	return computation
}

// Source: factorization.cpp:536-544. Print original A/B/C, exactly as
// the failed product check requests, before destroying test1 or br's owner.
func (s *UPolynomialFactorizationService) traceHenselPairCheckFailure(h *upolyHenselPairScope, A, B, C UPolynomialView) error {
	tags := s.manager.modular.BorrowTraceTags()
	if !tags.ShouldTrace("polynomial::factorization::bughunt") {
		return nil
	}
	if h == nil || h.self != h || h.service != s || !h.active {
		return ErrUPolynomialOwner
	}
	if e := h.traceBorrow.bindFamily([4]*UPolynomialManager{s.manager, h.rm, h.check.brCore, h.check.bCore}, tags); e != nil {
		return e
	}
	defer h.traceBorrow.unbind()
	var computation error
	_ = tags.EmitConditional("polynomial::factorization::bughunt", "check_hansel_lift", "src/math/polynomial/upolynomial_factorization.cpp", 536,
		func() bool { return true }, func(raw io.Writer) error {
			out := factorizationTraceWriter{raw}
			_ = linearWriteText(out, "sage: R.<x> = ZZ['x']")
			_ = diagnostic.WriteTraceEndl(out)
			for i, v := range [3]UPolynomialView{A, B, C} {
				_ = linearWriteText(out, [3]string{"sage: A = ", "sage: B = ", "sage: C = "}[i])
				if e := s.manager.displayIntoWithReceipt(v, factorizationTraceStream{out}, "x", false, &h.trace); e != nil {
					computation = e
					return e
				}
				_ = diagnostic.WriteTraceEndl(out)
			}
			_ = linearWriteText(out, "sage: test1 = C - AB")
			_ = diagnostic.WriteTraceEndl(out)
			_ = linearWriteText(out, "sage: print test1.change_ring(GF(")
			br, e := s.manager.config.Integer.String(&h.check.br)
			if e != nil {
				computation = e
				return e
			}
			_ = linearWriteText(out, br)
			_ = linearWriteText(out, "))")
			_ = diagnostic.WriteTraceEndl(out)
			_ = linearWriteText(out, "sage: print 'expected 0'")
			return diagnostic.WriteTraceEndl(out)
		})
	return computation
}
