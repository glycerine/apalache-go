package smtcore

// Source: src/smt/theory_pb.cpp weighted-resolution TRACE/IF_VERBOSE records
// and display_resolved_lemma. RTTI byte names use retained justification names.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (r *smtPBResolution) traceLemma(tag string, line int, heading string, values bool) error {
	return r.theory.emitPBTag(tag, "resolve_conflict", line, func(out io.Writer) error {
		_, _ = io.WriteString(out, heading)
		return r.theory.displayPBArgs(out, &r.lemma, values)
	})
}

func (r *smtPBResolution) displayResolvedLemma(out io.Writer) error {
	s := r.theory.state
	_, _ = fmt.Fprintf(out, "num marks: %d\nconflict level: %d\n", r.numMarks, r.level)
	for i, l := range s.assignedLiterals {
		level := s.booleanData[l.Var()].ScopeLevel
		_, marked := r.marks[l.Var()]
		mark := "u"
		if marked {
			mark = "m"
		}
		if err := smtPBWriteLiteral(out, l); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "@ %d %s\n", level, mark)
		if level == r.level && marked {
			_, _ = io.WriteString(out, "skipped: ")
			if err := smtPBWriteLiteral(out, l); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(out, ":%d\n", i)
		}
	}
	if err := r.theory.displayPBArgs(out, &r.lemma, true); err != nil {
		return err
	}
	var conflicts uint32
	for _, term := range r.lemma.terms {
		level := s.booleanData[term.literal.Var()].ScopeLevel
		if level == r.level {
			conflicts++
		}
		_, marked := r.marks[term.literal.Var()]
		mark := "u"
		if marked {
			mark = "m"
		}
		if err := smtPBWriteLiteral(out, term.literal); err != nil {
			return err
		}
		value, err := s.Assignment(term.literal)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "@%d %s %s\n", level, mark, value.String())
	}
	_, err := fmt.Fprintf(out, "num conflicts: %d\n", conflicts)
	return err
}

// C++ typeid names depend on its compiler ABI. Keep the concrete source
// barrier identity using the existing Go justification names, without an RTTI
// mangling adapter or new generic justification interface.
func smtPBJustificationTraceName(owner SMTJustification) string {
	if _, pb := owner.(*smtPBJustification); pb {
		return "pb-justification"
	}
	return owner.Name()
}

type smtPBVerboseWriter struct{ stream *utext.NumericTextStream }

func (w smtPBVerboseWriter) Write(p []byte) (int, error) {
	if err := w.stream.AppendString(string(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}

func smtPBVerbose(level uint32, body func(io.Writer) error) error {
	enabled, err := diagnostic.ProcessVerboseEnabled(level)
	if err != nil || !enabled {
		return err
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = body(smtPBVerboseWriter{stream}); err != nil {
		return err
	}
	return stream.Flush()
}
