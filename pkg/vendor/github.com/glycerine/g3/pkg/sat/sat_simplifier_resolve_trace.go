package sat

// Source: src/sat/sat_simplifier.cpp:1178,1241-1244; retained util/trace sites.

import (
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func satTraceClauseWrapper(a *ClauseAllocator, c satClauseWrapper) (string, error) {
	n, err := c.Size(a)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteByte('(')
	for i := uint32(0); i < n; i++ {
		if i != 0 {
			b.WriteByte(' ')
		}
		l, err := c.Literal(a, i)
		if err != nil {
			return "", err
		}
		b.WriteString(l.String())
	}
	b.WriteByte(')')
	return b.String(), nil
}

func (k *satSimplifierKernel) traceInvalidResolve(first, second satClauseWrapper, pivot Literal) {
	tags := diagnostic.BorrowProcessTraceTags()
	if k.lifecycle != nil && k.lifecycle.traceTags != nil {
		tags = k.lifecycle.traceTags
	}
	_ = tags.EmitConditional("resolve_bug", "resolve", "src/sat/sat_simplifier.cpp", 1178,
		func() bool {
			yes, err := first.ContainsLiteral(k.allocator, pivot)
			return err == nil && !yes
		}, func(w io.Writer) error {
			a, err := satTraceClauseWrapper(k.allocator, first)
			if err != nil {
				return err
			}
			b, err := satTraceClauseWrapper(k.allocator, second)
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, a+"\n"+b+"\nl: "+pivot.String()+"\n")
			return err
		})
}

func (s *satSimplifier) traceMissingReciprocalBinary(a, b Literal, first, second satSolverWatchListView, missing bool) {
	tags := s.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("resolve_bug", "add_non_learned_binary_clause", "src/sat/sat_simplifier.cpp", 1241,
		func() bool { return missing }, func(w io.Writer) error {
			out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: arena.NewBudget(^uint64(0))})
			if err != nil {
				return err
			}
			defer out.Close()
			if err = out.AppendCString(a.Negated().String() + " -> "); err != nil {
				return err
			}
			firstRef, err := first.ref()
			if err != nil {
				return err
			}
			if err = first.owner.store.DisplayInto(firstRef, s.lifecycle.propagation.clauses, out); err != nil {
				return err
			}
			if err = out.AppendCString("\n" + b.Negated().String() + " -> "); err != nil {
				return err
			}
			secondRef, err := second.ref()
			if err != nil {
				return err
			}
			if err = second.owner.store.DisplayInto(secondRef, s.lifecycle.propagation.clauses, out); err != nil {
				return err
			}
			if err = out.AppendCString("\n"); err != nil {
				return err
			}
			body, err := out.String()
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, body)
			return err
		})
}
