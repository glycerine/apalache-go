package sat

// Source: src/sat/sat_cleaner.cpp:86-137; src/util/trace.h:40-46.

import (
	"io"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (c *satCleaner) traceClauseRecord(tag string, line int, condition func() bool, body func(io.Writer) error) {
	tags := c.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional(tag, "cleanup_clauses", "src/sat/sat_cleaner.cpp", line, condition, body)
}

func (c *satCleaner) traceCleaningClause(r StoredClauseRef) {
	c.traceClauseRecord("sat_cleaner_bug", 86, func() bool { return true }, func(w io.Writer) error {
		a := c.lifecycle.propagation.clauses
		clause, err := satTraceClause(a, r)
		if err != nil {
			return err
		}
		var body strings.Builder
		body.WriteString("cleaning: ")
		body.WriteString(clause)
		body.WriteByte('\n')
		n, err := a.Size(r)
		if err != nil {
			return err
		}
		for i := uint32(0); i < n; i++ {
			lit, err := a.Literal(r, i)
			if err != nil {
				return err
			}
			value, err := c.lifecycle.propagation.variables.valueLiteral(lit)
			if err != nil {
				return err
			}
			body.WriteString(lit.String())
			body.WriteString(": ")
			body.WriteString(value.String())
			body.WriteByte('\n')
		}
		_, err = io.WriteString(w, body.String())
		return err
	})
}

func (c *satCleaner) traceFrozenClauseBefore(r StoredClauseRef) {
	var frozen bool
	c.traceClauseRecord("sat_cleaner_frozen", 88, func() bool {
		var err error
		frozen, err = c.lifecycle.propagation.clauses.Frozen(r)
		return err == nil && frozen
	}, func(w io.Writer) error {
		clause, err := satTraceClause(c.lifecycle.propagation.clauses, r)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, clause+"\n")
		return err
	})
}

func (c *satCleaner) traceFrozenClauseAfter(r StoredClauseRef, satisfied bool, size uint32) {
	c.traceClauseRecord("sat_cleaner_frozen", 108, func() bool {
		frozen, err := c.lifecycle.propagation.clauses.Frozen(r)
		return err == nil && frozen
	}, func(w io.Writer) error {
		lits, err := satTraceClauseLiterals(c.lifecycle.propagation.clauses, r, size)
		if err != nil {
			return err
		}
		flag := "0"
		if satisfied {
			flag = "1"
		}
		_, err = io.WriteString(w, "sat: "+flag+", new_size: "+strconv.FormatUint(uint64(size), 10)+"\n"+lits+"\n")
		return err
	})
}

func (c *satCleaner) traceSmallClause(r StoredClauseRef, size uint32) {
	c.traceClauseRecord("sat_cleaner_bug", 117, func() bool { return size < 2 }, func(w io.Writer) error {
		a := c.lifecycle.propagation.clauses
		body := "new_sz: " + strconv.FormatUint(uint64(size), 10) + "\n"
		n, err := a.Size(r)
		if err != nil {
			return err
		}
		if n > 0 {
			lit, err := a.Literal(r, 0)
			if err != nil {
				return err
			}
			body += "unit: " + lit.String() + "\n"
		}
		_, err = io.WriteString(w, body)
		return err
	})
}

func (c *satCleaner) traceBecameBinary(r StoredClauseRef) {
	c.traceClauseRecord("cleanup_bug", 137, func() bool { return true }, func(w io.Writer) error {
		a := c.lifecycle.propagation.clauses
		first, err := a.Literal(r, 0)
		if err != nil {
			return err
		}
		second, err := a.Literal(r, 1)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, "clause became binary: "+first.String()+" "+second.String()+"\n")
		return err
	})
}
