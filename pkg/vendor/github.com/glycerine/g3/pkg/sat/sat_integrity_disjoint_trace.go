package sat

// Source: src/sat/sat_integrity_checker.cpp:220-222; src/util/trace.h:40-46.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (c satIntegrityChecker) traceRepeatedClause(id uint32) {
	tags := c.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("sat") {
		return
	}
	_ = tags.EmitConditional("sat", "check_disjoint_clauses", "src/sat/sat_integrity_checker.cpp", 221,
		func() bool { return true }, func(w io.Writer) error {
			_, e := io.WriteString(w, "Repeated clause: "+strconv.FormatUint(uint64(id), 10)+"\n")
			return e
		})
}
