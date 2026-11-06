package sat

// Source: src/sat/sat_asymm_branch.cpp:68,118,143,162,187; retained util/trace sites.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (a *satAsymmBranch) trace(tag, function string, line int, body func(io.Writer) error) {
	tags := a.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional(tag, function, "src/sat/sat_asymm_branch.cpp", line,
		func() bool { return true }, body)
}

func (a *satAsymmBranch) traceSolverDisplay() {
	s := a.solver
	if s == nil {
		return
	}
	a.trace("asymm_branch_detail", "operator()", 68, s.traceUserDisplayInto)
}

func (a *satAsymmBranch) traceProcessing(c StoredClauseRef) {
	a.trace("asymm_branch_detail", "process", 118, func(w io.Writer) error {
		text, err := satTraceClause(a.lifecycle.propagation.clauses, c)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, "processing: "+text+"\n")
		return err
	})
}

func (a *satAsymmBranch) traceAssign(l Literal) {
	a.trace("asymm_branch_detail", "process", 143, func(w io.Writer) error {
		_, err := io.WriteString(w, "assigning: "+l.String()+"\n")
		return err
	})
}

func (a *satAsymmBranch) traceReduced(c StoredClauseRef, newSize uint32) {
	a.trace("asymm_branch", "process", 162, func(w io.Writer) error {
		cl := a.lifecycle.propagation.clauses
		text, err := satTraceClause(cl, c)
		if err != nil {
			return err
		}
		if _, err = io.WriteString(w, text+"\nnew_size: "+strconv.FormatUint(uint64(newSize), 10)+"\n"); err != nil {
			return err
		}
		size, err := cl.Size(c)
		if err != nil {
			return err
		}
		for i := uint32(0); i < size; i++ {
			l, err := cl.Literal(c, i)
			if err != nil {
				return err
			}
			value, err := a.lifecycle.propagation.variables.valueLiteral(l)
			if err != nil {
				return err
			}
			if _, err = io.WriteString(w, strconv.FormatInt(int64(value), 10)+" "); err != nil {
				return err
			}
		}
		_, err = io.WriteString(w, "\n")
		return err
	})
}

func (a *satAsymmBranch) traceUnit(l Literal) {
	a.trace("asymm_branch", "process", 187, func(w io.Writer) error {
		_, err := io.WriteString(w, "produced unit clause: "+l.String()+"\n")
		return err
	})
}
