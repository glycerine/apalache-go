package polynomial

// Source: upolynomial.cpp:391-392; mpz.cpp:1432-1435 (to_string).

import (
	"bytes"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"io"
	"strconv"
)

type upolyNormalizeStringStream struct{ text *bytes.Buffer }

func (s upolyNormalizeStringStream) AppendCString(v string) error {
	_, e := s.text.WriteString(v)
	return e
}
func (s upolyNormalizeStringStream) AppendNumeralText(v string) error { return s.AppendCString(v) }
func (s upolyNormalizeStringStream) AppendInt32(v int32) error {
	return s.AppendCString(strconv.FormatInt(int64(v), 10))
}

// CTRACE's condition and all four to_string operands are lazy. Go buffers
// replace ostringstream; project numeral-formatting receipts remain explicit.
func (q *upolyRootIDScope) traceNormalization(x *unumeral.Int) error {
	m, s := q.owner, &q.normalization
	tags := m.config.Integer.BorrowTraceTags()
	if !tags.ShouldTrace("div_bug") {
		return nil
	}
	if q.context().rootIDFrame(m.config.Integer) != q || !s.executing || s.constructed != 3 {
		return ErrUPolynomialOwner
	}
	var computation error
	m.callbackDepth++
	defer func() { m.callbackDepth-- }()
	_ = tags.EmitConditional("div_bug", "normalize", "src/math/polynomial/upolynomial.cpp", 391, func() bool {
		equal, e := m.modular.Equal(&s.values[2], &s.values[1])
		computation = e
		return e == nil && !equal
	}, func(w io.Writer) error {
		values := [4]*unumeral.Int{&s.values[1], &s.values[0], x, &s.values[2]}
		labels := [4]string{"old(p[i]): ", ", g: ", ", p[i]: ", ", tmp: "}
		for i, a := range values {
			_, _ = io.WriteString(w, labels[i])
			var text bytes.Buffer
			if e := m.config.Integer.InsertSourceIntegerTracked(upolyNormalizeStringStream{&text}, a, &s.insertions[i], nil); e != nil {
				computation = e
				return e
			}
			_, _ = io.WriteString(w, text.String())
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	})
	return computation
}
