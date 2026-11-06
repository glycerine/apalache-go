package sat

// Source: src/sat/sat_solver.cpp:940,989; retained util/trace TRACE sites.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (d *satSolverDecision) traceRecord(tag, function string, line int, body func(io.Writer) error) {
	tags := d.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional(tag, function, "src/sat/sat_solver.cpp", line,
		func() bool { return true }, body)
}

func (d *satSolverDecision) traceRandomSplit(next BoolVar) {
	d.traceRecord("random_split", "next_var", 940, func(w io.Writer) error {
		value, err := d.conflict.analysis.propagation.variables.valueVar(next)
		if err != nil {
			return err
		}
		_, _ = io.WriteString(w, "next: "+strconv.FormatUint(uint64(next), 10)+
			" value(next): "+value.String()+"\n")
		return nil
	})
}

func (d *satSolverDecision) traceDecide(next Literal) {
	d.traceRecord("sat_decide", "decide", 989, func(w io.Writer) error {
		level := d.conflict.analysis.propagation.variables.scopeLevel
		_, _ = io.WriteString(w, strconv.FormatUint(uint64(level), 10)+
			": next-case-split: "+next.String()+"\n")
		return nil
	})
}
