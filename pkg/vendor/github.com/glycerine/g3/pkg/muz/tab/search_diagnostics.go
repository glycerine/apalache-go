package tab

// Source: src/muz/tab/tab_context.cpp imp::query and select_predicate.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// Source ostream refusal sets badbit without interrupting later operands.
// Formatting and AST-construction errors still propagate from their callers.
type tabDiagnosticWriter struct{ stream *utext.NumericTextStream }

func (w tabDiagnosticWriter) Write(body []byte) (int, error) {
	_ = w.stream.AppendCString(string(body))
	return len(body), nil
}

func tabVerboseStream(minimum uint32) (tabDiagnosticWriter, bool, error) {
	enabled, err := diagnostic.ProcessVerboseEnabled(minimum)
	if err != nil || !enabled {
		return tabDiagnosticWriter{}, false, err
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	return tabDiagnosticWriter{stream: stream}, err == nil, err
}

func (s *tabSearch) verboseInitialClause() error {
	out, enabled, err := tabVerboseStream(1)
	if err != nil || !enabled {
		return err
	}
	clause := s.stack[len(s.stack)-1]
	_, _ = fmt.Fprintf(out, "g%d ", clause.sequence)
	return clause.Display(out)
}

func (s *tabSearch) verboseSelectedPredicate(clause *Clause) error {
	out, enabled, err := tabVerboseStream(2)
	if err != nil || !enabled {
		return err
	}
	predicate, err := s.manager.RefGet(clause.predicates[clause.predicateIndex])
	if err != nil {
		return err
	}
	if err = ast.DisplayLegacySMT(out, s.manager, predicate, 0, nil); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(out)
	return nil
}

func (s *tabSearch) verboseEmptyClause(clause *Clause) error {
	out, enabled, err := tabVerboseStream(2)
	if err != nil || !enabled {
		return err
	}
	return clause.Display(out)
}
