package ast

// Source: src/ast/rewriter/fpa_rewriter.cpp:143,159,168,177,179,195,209,226;
// src/util/trace.h:40-46.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// TraceTags supplies a sticky ostream. A failed insertion suppresses later
// physical writes, while reached numeral operands still evaluate in order.
func writeFPATracePart(w io.Writer, part string) { _, _ = io.WriteString(w, part) }
func endFPATraceLine(w io.Writer)                { _ = diagnostic.WriteTraceEndl(w) }

func (m *ASTManager) emitFPAToFPTrace(line int, body func(io.Writer) error) error {
	tags := m.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("fp_rewriter") {
		return nil
	}
	err := tags.EmitConditional("fp_rewriter", "mk_to_fp", "src/ast/rewriter/fpa_rewriter.cpp", line,
		func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (m *ASTManager) traceFPAToFPRational(line int, label string, r *unumeral.OwnedRational) error {
	return m.emitFPAToFPTrace(line, func(w io.Writer) error {
		writeFPATracePart(w, label)
		value, e := m.numerals.String(r)
		if e != nil {
			return e
		}
		writeFPATracePart(w, value)
		endFPATraceLine(w)
		return nil
	})
}

func (m *ASTManager) traceFPAToFPArithmeticInput(r *unumeral.OwnedRational) error {
	return m.traceFPAToFPRational(159, "r: ", r)
}

func (m *ASTManager) traceFPAToFPRationalPair(line int, first, second *unumeral.OwnedRational) error {
	return m.emitFPAToFPTrace(line, func(w io.Writer) error {
		writeFPATracePart(w, "r1: ")
		left, e := m.numerals.String(first)
		if e != nil {
			return e
		}
		writeFPATracePart(w, left)
		writeFPATracePart(w, ", r2: ")
		right, e := m.numerals.String(second)
		if e != nil {
			return e
		}
		writeFPATracePart(w, right)
		writeFPATracePart(w, "\n")
		return nil
	})
}

func (m *ASTManager) traceFPAToFPRawRational(line int, label string, r *unumeral.Rational) error {
	return m.emitFPAToFPTrace(line, func(w io.Writer) error {
		writeFPATracePart(w, label)
		qm, e := m.floatingPointPlugin.numeric.BorrowRationalManager()
		if e != nil {
			return e
		}
		value, e := qm.String(r)
		if e != nil {
			return e
		}
		writeFPATracePart(w, value)
		endFPATraceLine(w)
		return nil
	})
}

func (m *ASTManager) traceFPAToFPValue(line int, label string, v *unumeral.ArbitraryFloat) error {
	return m.emitFPAToFPTrace(line, func(w io.Writer) error {
		writeFPATracePart(w, label)
		value, e := m.floatingPointPlugin.numeric.ToString(v)
		if e != nil {
			return e
		}
		writeFPATracePart(w, value)
		endFPATraceLine(w)
		return nil
	})
}

func (m *ASTManager) traceFPAToFPIEEEBVFields(negative bool, sig *unumeral.Int, exponent int64, v *unumeral.ArbitraryFloat) error {
	return m.emitFPAToFPTrace(143, func(w io.Writer) error {
		sign := "0"
		if negative {
			sign = "1"
		}
		writeFPATracePart(w, "sgn: ")
		writeFPATracePart(w, sign)
		endFPATraceLine(w)
		writeFPATracePart(w, "sig: ")
		owner, e := m.floatingPointPlugin.numeric.BorrowIntegerManager()
		if e != nil {
			return e
		}
		significand, e := owner.String(sig)
		if e != nil {
			return e
		}
		writeFPATracePart(w, significand)
		endFPATraceLine(w)
		writeFPATracePart(w, "exp: ")
		writeFPATracePart(w, strconv.FormatInt(exponent, 10))
		endFPATraceLine(w)
		writeFPATracePart(w, "v: ")
		value, e := m.floatingPointPlugin.numeric.ToString(v)
		if e != nil {
			return e
		}
		writeFPATracePart(w, value)
		endFPATraceLine(w)
		return nil
	})
}
