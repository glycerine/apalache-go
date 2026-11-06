package sat

// Source: src/sat/sat_solver.cpp:1080-1086; retained util/trace TRACE site.

import (
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (s *Solver) traceModelConverterInto(w io.Writer) error {
	b := arena.NewBudget(^uint64(0))
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: b})
	if err != nil {
		return err
	}
	defer out.Close()
	if err = s.converter.DisplayInto(out); err != nil {
		return err
	}
	value, err := out.String()
	if err != nil {
		return err
	}
	_, _ = io.WriteString(w, value)
	return nil
}

func (s *Solver) traceInitAssumptions(count uint32, span satSolverLiteralSpan) {
	tags := s.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("sat", "init_assumptions", "src/sat/sat_solver.cpp", 1080,
		func() bool { return true }, func(w io.Writer) error {
			var b strings.Builder
			for i := uint32(0); i < count; i++ {
				l, err := span.literal(i)
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
			n, err := s.conflict.userScopeLiterals.Size()
			if err != nil {
				return err
			}
			if n != 0 {
				b.Reset()
				b.WriteString("user literals: ")
				for i := uint32(0); i < n; i++ {
					v, err := s.conflict.userScopeLiterals.Read(i)
					if err != nil {
						return err
					}
					if i != 0 {
						b.WriteByte(' ')
					}
					b.WriteString(Literal(v).String())
				}
				b.WriteByte('\n')
				_, _ = io.WriteString(w, b.String())
			}
			return s.traceModelConverterInto(w)
		})
}
