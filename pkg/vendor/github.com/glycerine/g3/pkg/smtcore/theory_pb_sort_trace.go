package smtcore

// Source: src/util/sorting_network.h, concrete retained SMTLiteral psort.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (s *smtPBSort) require(condition bool) bool {
	if smtcoreSourceDebug && !condition && s.err == nil {
		s.err = ast.ErrASTDomain
	}
	return s.err == nil
}

func (s *smtPBSort) trace(function string, line int, body func(io.Writer) error) {
	if s.err != nil {
		return
	}
	tags := s.theory.pbTraceTags()
	var operandErr error
	err := tags.EmitConditional("pb", function, "src/util/sorting_network.h", line, func() bool { return true }, func(out io.Writer) error {
		operandErr = body(assertedTraceWriter{out})
		return operandErr
	})
	if operandErr != nil {
		s.err = operandErr
	} else if !tags.IsStreamFailure(err) {
		s.err = err
	}
}

func smtPBSortTraceVector(out io.Writer, heading string, xs []SMTLiteral) error {
	_, _ = io.WriteString(out, heading)
	if err := smtPBWriteLiterals(out, xs, true); err != nil {
		return err
	}
	_, err := io.WriteString(out, "\n")
	return err
}

func (s *smtPBSort) traceCard(line int, k, n uint32, xs, ys []SMTLiteral, complete bool) {
	s.trace("card", line, func(out io.Writer) error {
		_, _ = fmt.Fprintf(out, "card k: %d n: %d\n", k, n)
		if complete {
			if err := smtPBSortTraceVector(out, "in:", xs); err != nil {
				return err
			}
			return smtPBSortTraceVector(out, "out:", ys)
		}
		return nil
	})
}

func (s *smtPBSort) traceSorting(line int, xs, ys []SMTLiteral, complete bool) {
	s.trace("sorting", line, func(out io.Writer) error {
		_, _ = fmt.Fprintf(out, "sorting: %d\n", len(xs))
		if complete {
			if err := smtPBSortTraceVector(out, "in:", xs); err != nil {
				return err
			}
			return smtPBSortTraceVector(out, "out:", ys)
		}
		return nil
	})
}

func (s *smtPBSort) traceMerge(function string, line int, c uint32, as, bs, ys []SMTLiteral, complete bool) {
	s.trace(function, line, func(out io.Writer) error {
		if function == "merge" {
			_, _ = fmt.Fprintf(out, "merge a: %d b: %d\n", len(as), len(bs))
		} else if function == "interleave" {
			_, _ = fmt.Fprintf(out, "interleave: %d %d\n", len(as), len(bs))
		} else {
			_, _ = fmt.Fprintf(out, "%s: c:%d a:%d b:%d\n", function, c, len(as), len(bs))
		}
		if complete {
			space := ""
			if function == "interleave" {
				space = " "
			}
			if err := smtPBSortTraceVector(out, "a:"+space, as); err != nil {
				return err
			}
			if err := smtPBSortTraceVector(out, "b:"+space, bs); err != nil {
				return err
			}
			return smtPBSortTraceVector(out, "out:"+space, ys)
		}
		return nil
	})
}

func (s *smtPBSort) dualize(k, n uint32, xs []SMTLiteral) []SMTLiteral {
	if !s.require(k <= n) {
		return nil
	}
	in := smtPBSortNot(xs)
	s.trace("dualize", 516, func(out io.Writer) error {
		_, _ = fmt.Fprintf(out, "%d: ", n)
		if err := smtPBWriteLiterals(out, in, true); err != nil {
			return err
		}
		_, err := fmt.Fprintf(out, " ~ %d\n", n-k)
		return err
	})
	return in
}
