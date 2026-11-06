package sat

// Source: src/sat/sat_solver.cpp:2625,2741; retained util/trace TRACE sites.

import (
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (s *Solver) traceUserScopeRecord(function string, line int, body func(io.Writer) error) {
	tags := s.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("sat", function, "src/sat/sat_solver.cpp", line,
		func() bool { return true }, body)
}

func (s *Solver) traceUserPush(l Literal) {
	s.traceUserScopeRecord("user_push", 2625, func(w io.Writer) error {
		_, _ = io.WriteString(w, "user_push: "+l.String()+"\n")
		return nil
	})
}

func (s *Solver) traceUserPopTrail() {
	s.traceUserScopeRecord("user_pop", 2741, func(w io.Writer) error {
		n, err := s.Variables.trail.Size(s.Variables.trailRef)
		if err != nil {
			return err
		}
		var b strings.Builder
		for i := uint32(0); i < n; i++ {
			l, err := s.Variables.trailRead(i)
			if err != nil {
				return err
			}
			if i != 0 {
				b.WriteByte(' ')
			}
			b.WriteString(l.String())
		}
		b.WriteByte('\n')
		_, _ = io.WriteString(w, b.String())
		return nil
	})
}

func (s *Solver) traceUserDisplayInto(w io.Writer) error {
	// The source trace stream is outside the solver's custom memory budget.
	b := arena.NewBudget(^uint64(0))
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: b})
	if err != nil {
		return err
	}
	defer out.Close()
	if err = s.displayInto(out); err != nil {
		return err
	}
	text, err := out.String()
	if err != nil {
		return err
	}
	_, _ = io.WriteString(w, text)
	return nil
}

func (s *Solver) traceUserAssignmentInto(w io.Writer) error {
	b := arena.NewBudget(^uint64(0))
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: b})
	if err != nil {
		return err
	}
	defer out.Close()
	if err = s.displayAssignment(out); err != nil {
		return err
	}
	text, err := out.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, text)
	return err
}

func (s *Solver) traceUserPopDisplay() {
	s.traceUserScopeRecord("user_pop", 2727, s.traceUserDisplayInto)
}

func (s *Solver) traceUserPopGC(l Literal) {
	s.traceUserScopeRecord("user_pop", 2737, func(w io.Writer) error {
		_, _ = io.WriteString(w, "gc: "+l.String()+"\n")
		return s.traceUserDisplayInto(w)
	})
}
