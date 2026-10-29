package sat

// Source: src/sat/sat_solver.cpp:1494,1519; retained util/trace TRACE sites.

import (
	"io"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (g *satSolverGC) traceGC(function string, line int) {
	tags := g.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("sat", function, "src/sat/sat_solver.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "gc\n")
			return nil
		})
}

// Source: src/sat/sat_solver.cpp:1569,1612,1630. The clause renderer follows
// src/sat/sat_clause.cpp:221-231; the cleanup site prints only the live prefix.
func satTraceClauseLiterals(a *ClauseAllocator, r StoredClauseRef, n uint32) (string, error) {
	var b strings.Builder
	for i := uint32(0); i < n; i++ {
		lit, err := a.Literal(r, i)
		if err != nil {
			return "", err
		}
		if i != 0 {
			b.WriteByte(' ')
		}
		b.WriteString(lit.String())
	}
	return b.String(), nil
}

func satTraceClause(a *ClauseAllocator, r StoredClauseRef) (string, error) {
	n, err := a.Size(r)
	if err != nil {
		return "", err
	}
	text, err := satTraceClauseLiterals(a, r, n)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteByte('(')
	b.WriteString(text)
	b.WriteByte(')')
	for _, f := range []struct {
		mask uint32
		mark byte
	}{{2, 'x'}, {1, '+'}, {4, '*'}} {
		yes, err := a.flag(r, f.mask)
		if err != nil {
			return "", err
		}
		if yes {
			b.WriteByte(f.mark)
		}
	}
	return b.String(), nil
}

func (g *satSolverGC) traceFreezingClause(r StoredClauseRef, size, measure uint32) {
	g.traceClauseRecord("sat_frozen", "gc_dyn_psm", 1569, func(w io.Writer) error {
		clause, err := satTraceClause(g.lifecycle().propagation.clauses, r)
		if err != nil {
			return err
		}
		_, _ = io.WriteString(w, "freezing size: "+strconv.FormatUint(uint64(size), 10)+
			" psm: "+strconv.FormatUint(uint64(measure), 10)+" "+clause+"\n")
		return nil
	})
}

func (g *satSolverGC) traceReactivatingClause(r StoredClauseRef) {
	g.traceClauseRecord("sat_gc", "activate_frozen_clause", 1612, func(w io.Writer) error {
		clause, err := satTraceClause(g.lifecycle().propagation.clauses, r)
		if err != nil {
			return err
		}
		_, _ = io.WriteString(w, "reactivating:\n"+clause+"\n")
		return nil
	})
}

func (g *satSolverGC) traceAfterClauseCleanup(r StoredClauseRef, n uint32) {
	g.traceClauseRecord("sat", "activate_frozen_clause", 1630, func(w io.Writer) error {
		lits, err := satTraceClauseLiterals(g.lifecycle().propagation.clauses, r, n)
		if err != nil {
			return err
		}
		_, _ = io.WriteString(w, "after cleanup:\n"+lits+"\n")
		return nil
	})
}

func (g *satSolverGC) traceClauseRecord(tag, function string, line int, body func(io.Writer) error) {
	tags := g.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional(tag, function, "src/sat/sat_solver.cpp", line,
		func() bool { return true }, body)
}

func (g *satSolverGC) traceMinDTk() {
	tags := g.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("sat_frozen", "gc_dyn_psm", "src/sat/sat_solver.cpp", 1540,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "m_min_d_tk: ")
			_, _ = io.WriteString(w, strconv.FormatFloat(g.minDTk, 'g', 6, 64))
			_, _ = io.WriteString(w, "\n")
			return nil
		})
}
