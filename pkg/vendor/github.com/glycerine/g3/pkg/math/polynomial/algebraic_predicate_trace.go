package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:290,302,310;
// src/util/mpz.cpp:1432-1435, mpbq.cpp:384-392. Strings use Go buffers.

import (
	"bytes"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"io"
	"strconv"
)

type algebraicPredicateTraceScope struct {
	root       algebraicRootDisplayInvocationReceipt
	owner      *AlgebraicRootCore
	invocation uint64
	insertions [3]unumeral.NumeralInsertionReceipt // coefficient, lower, upper
	active     bool
	stop       error
}
type algebraicPredicateTraceInvocation struct {
	owner      *AlgebraicRootCore
	frame      *algebraicPredicateTraceScope
	invocation uint64
}
type algebraicPredicateTraceStream struct{ io.Writer }

func (s algebraicPredicateTraceStream) AppendCString(v string) error {
	_, _ = io.WriteString(s.Writer, v)
	return nil
}
func (s algebraicPredicateTraceStream) AppendNumeralText(v string) error { return s.AppendCString(v) }
func (s algebraicPredicateTraceStream) AppendInt32(v int32) error {
	return s.AppendCString(strconv.FormatInt(int64(v), 10))
}
func (s *algebraicPredicateTraceScope) pending() bool {
	if s.owner.rootDisplayInvocationPending(&s.root) {
		return true
	}
	for i := range s.insertions {
		if s.insertions[i].Active() {
			return true
		}
	}
	return s.stop != nil
}
func (m *AlgebraicRootCore) predicateTracePending(r *algebraicPredicateTraceInvocation) bool {
	s := &m.predicateTrace
	return r.owner == m && r.frame == s && s.owner == m && r.invocation != 0 && r.invocation == s.invocation && s.active
}
func (m *AlgebraicRootCore) cleanupPredicateTrace() error {
	s := &m.predicateTrace
	if !s.active {
		return nil
	}
	if s.owner != m || m.self != m {
		return ErrAlgebraicOwner
	}
	if m.predicateDiagnosticRunning {
		return ErrAlgebraicPending
	}
	m.predicateDiagnosticRunning = true
	defer func() { m.predicateDiagnosticRunning = false }()
	for i := len(s.insertions) - 1; i >= 0; i-- {
		r := &s.insertions[i]
		if !r.Active() {
			continue
		}
		if e := r.Complete(m.config.Rationals.Integers().Natural, false); e != nil {
			return e
		}
		if e := r.Complete(m.config.Rationals.Integers().Natural, true); e != nil {
			return e
		}
	}
	if e := m.cleanupRootDisplayInvocation(&s.root); e != nil {
		return e
	}
	s.active = false
	return nil
}

// The numeric producer owns its exact character/formatter receipt. Go's
// buffer replaces ostringstream; only the project numeral algorithm is ported.
func (m *AlgebraicRootCore) predicateIntegerString(a *unumeral.Int, r *unumeral.NumeralInsertionReceipt) (string, error) {
	var text bytes.Buffer
	e := m.config.Rationals.Integers().InsertSourceIntegerTracked(algebraicPredicateTraceStream{&text}, a, r, nil)
	return text.String(), e
}
func (m *AlgebraicRootCore) predicateIntervalTrace(out algebraicPredicateTraceStream, l *algebraicLocation) error {
	return m.displayIntervalAtLocationWithReceipts(out.Writer, l, &m.predicateTrace.insertions[1], &m.predicateTrace.insertions[2])
}
func (m *AlgebraicRootCore) traceRationalPredicate(a *Algebraic, phase uint8, k uint32) error {
	tags := m.config.Rationals.Integers().BorrowTraceTags()
	if !tags.ShouldTrace("algebraic_bug") {
		return nil
	}
	if e := m.check(); e != nil {
		return e
	}
	s := &m.predicateTrace
	if s.active || s.invocation == ^uint64(0) {
		return ErrAlgebraicPending
	}
	if phase > 0 && !m.predicate.active {
		return ErrAlgebraicPending
	}
	*s = algebraicPredicateTraceScope{owner: m, invocation: s.invocation + 1}
	if phase > 0 {
		m.predicate.trace = algebraicPredicateTraceInvocation{m, s, s.invocation}
	}
	m.predicateDiagnosticRunning = true
	defer func() {
		m.predicateDiagnosticRunning = false
		for i := range s.insertions {
			if s.stop == nil {
				s.stop = s.insertions[i].SourceCause()
			}
		}
		if s.stop == nil && m.matchesRootDisplayInvocation(&s.root) {
			s.stop = m.rootDisplay.sourceStop
		}
		s.active = s.pending()
		if phase > 0 && s.stop != nil {
			m.predicate.childStop = s.stop
		}
	}()
	line := 290
	if phase == 1 {
		line = 302
	} else if phase == 2 {
		line = 310
	}
	var computation error
	_ = tags.EmitConditional("algebraic_bug", "is_rational", "src/math/polynomial/algebraic_numbers.cpp", line, func() bool { return true }, func(w io.Writer) error {
		out := algebraicPredicateTraceStream{algebraicComparisonTraceWriter{w}}
		l, e := m.locationCleanup(a)
		if e != nil {
			computation = e
			return e
		}
		switch phase {
		case 0:
			_ = out.AppendCString("is_rational(a):\n")
			e = m.displayRootAtLocationWithReceipt(out, l, false, &s.root)
			if e == nil {
				_ = out.AppendCString("\n")
				e = m.predicateIntervalTrace(out, l)
			}
		case 1:
			_ = out.AppendCString("abs(an): ")
			var text string
			text, e = m.predicateIntegerString(&m.isRationalTmp, &s.insertions[0])
			if e == nil {
				_ = out.AppendCString(text)
				_ = out.AppendCString(", k: ")
				_ = out.AppendCString(strconv.FormatUint(uint64(k), 10))
			}
		case 2:
			_ = out.AppendCString("interval after refinement: ")
			e = m.predicateIntervalTrace(out, l)
		}
		if e != nil {
			computation = e
			return e
		}
		_ = out.AppendCString("\n")
		return nil
	})
	return computation
}
