package sat

// Source: src/sat/sat_elim_eqs.cpp:81,104,138,187,205,216; retained util/trace sites.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (e *satElimEqs) trace(tag, function string, line int, condition func() bool, body func(io.Writer) error) {
	tags := e.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional(tag, function, "src/sat/sat_elim_eqs.cpp", line, condition, body)
}

func (e *satElimEqs) traceClause(line int, prefix string, c StoredClauseRef, suffix string) {
	e.trace("elim_eqs", "cleanup_clauses", line, func() bool { return true }, func(w io.Writer) error {
		body, err := satTraceClause(e.lifecycle.propagation.clauses, c)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, prefix+body+suffix+"\n")
		return err
	})
}

func (e *satElimEqs) traceSaveElim(v BoolVar, l, r Literal) {
	e.trace("save_elim", "save_elim", 187, func() bool { return true }, func(w io.Writer) error {
		_, err := io.WriteString(w, "marking as deleted: "+strconv.FormatUint(uint64(v), 10)+" l: "+l.String()+" r: "+r.String()+"\n")
		return err
	})
}

func (e *satElimEqs) traceEliminatedClauseLiteral(eliminated bool, lit Literal, c StoredClauseRef) {
	e.trace("elim_eqs_bug", "check_clauses", 205, func() bool { return eliminated }, func(w io.Writer) error {
		r, err := e.norm(lit)
		if err != nil {
			return err
		}
		body, err := satTraceClause(e.lifecycle.propagation.clauses, c)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, "lit: "+lit.String()+" "+r.String()+"\n"+body+"\n")
		return err
	})
}

func (e *satElimEqs) traceBinCleanup() {
	if e.solver == nil {
		return
	}
	e.trace("elim_eqs", "operator()", 216, func() bool { return true }, func(w io.Writer) error {
		if _, err := io.WriteString(w, "after bin cleanup\n"); err != nil {
			return err
		}
		return e.solver.traceUserDisplayInto(w)
	})
}
