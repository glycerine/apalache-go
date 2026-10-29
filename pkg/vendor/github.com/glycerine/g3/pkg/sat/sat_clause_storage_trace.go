package sat

// Source: src/sat/sat_clause.cpp:203,209; retained util/trace sites.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (a *ClauseAllocator) traceClauseStorage(function string, line int, r StoredClauseRef, learned bool) {
	if a.temporary {
		return
	}
	tags := a.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("sat", function, "src/sat/sat_clause.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			id, err := a.ID(r)
			if err != nil {
				return err
			}
			clause, err := satTraceClause(a, r)
			if err != nil {
				return err
			}
			body := "delete: " + strconv.FormatUint(uint64(id), 10) + " " + clause + "\n"
			if line == 203 {
				kind := "a"
				if learned {
					kind = "l"
				}
				body = "alloc: " + strconv.FormatUint(uint64(id), 10) + " " + clause + " " + kind + "\n"
			}
			_, err = io.WriteString(w, body)
			return err
		})
}

func (a *ClauseAllocator) traceAllocated(r StoredClauseRef, learned bool) {
	a.traceClauseStorage("mk_clause", 203, r, learned)
}

func (a *ClauseAllocator) traceDeleted(r StoredClauseRef) {
	a.traceClauseStorage("del_clause", 209, r, false)
}
