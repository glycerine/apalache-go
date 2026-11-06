package sat

// Source: src/sat/sat_simplifier.cpp:1105-1401; retained util/trace sites.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *satSimplifier) traceElimination(tag, method string, line int, body func(io.Writer) error) {
	tags := s.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional(tag, method, "src/sat/sat_simplifier.cpp", line,
		func() bool { return true }, body)
}

func (s *satSimplifier) traceEliminationProcessing(v BoolVar) {
	s.traceElimination("resolution_bug", "try_eliminate", 1302, func(w io.Writer) error {
		_, err := io.WriteString(w, "processing: "+strconv.FormatUint(uint64(v), 10)+"\n")
		return err
	})
}

func (s *satSimplifier) traceEliminationZeroCost(v BoolVar, pos, neg, binPos, binNeg, cost uint32) {
	if cost != 0 {
		return
	}
	s.traceElimination("elim_vars_detail", "get_to_elim_cost", 1105, func(w io.Writer) error {
		_, err := io.WriteString(w, strconv.FormatUint(uint64(v), 10)+
			" num_pos: "+strconv.FormatUint(uint64(pos), 10)+
			" num_neg: "+strconv.FormatUint(uint64(neg), 10)+
			" num_bin_pos: "+strconv.FormatUint(uint64(binPos), 10)+
			" num_bin_neg: "+strconv.FormatUint(uint64(binNeg), 10)+
			" cost: "+strconv.FormatUint(uint64(cost), 10)+"\n")
		return err
	})
}

func (s *satSimplifier) traceEliminationSortedCosts(costs []satSimplifierVarCost) {
	s.traceElimination("elim_vars", "order_vars_for_elim", 1133, func(w io.Writer) error {
		for _, c := range costs {
			if _, err := io.WriteString(w, "("+strconv.FormatUint(uint64(c.variable), 10)+", "+
				strconv.FormatUint(uint64(c.cost), 10)+") "); err != nil {
				return err
			}
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}

func (s *satSimplifier) traceRemovingBinary(selected, other Literal) {
	s.traceElimination("bin_clause_bug", "remove_bin_clauses", 1269, func(w io.Writer) error {
		_, err := io.WriteString(w, "removing: "+selected.String()+" "+other.String()+"\n")
		return err
	})
}

func (s *satSimplifier) traceCollapsingBinaryWatch(l Literal) {
	s.traceElimination("bin_clause_bug", "remove_bin_clauses", 1280, func(w io.Writer) error {
		_, err := io.WriteString(w, "collapsing watch_list of: "+l.String()+"\n")
		return err
	})
}

func (s *satSimplifier) traceEliminationOccurrences(v BoolVar, pos, neg uint32) {
	s.traceElimination("resolution", "try_eliminate", 1315, func(w io.Writer) error {
		_, err := io.WriteString(w, strconv.FormatUint(uint64(v), 10)+
			" num_pos: "+strconv.FormatUint(uint64(pos), 10)+
			" neg_pos: "+strconv.FormatUint(uint64(neg), 10)+"\n")
		return err
	})
}

func (s *satSimplifier) traceEliminationBeforeLiterals(v BoolVar, pos, neg, before uint32) {
	s.traceElimination("resolution", "try_eliminate", 1338, func(w io.Writer) error {
		_, err := io.WriteString(w, strconv.FormatUint(uint64(v), 10)+
			" num_pos: "+strconv.FormatUint(uint64(pos), 10)+
			" neg_pos: "+strconv.FormatUint(uint64(neg), 10)+
			" before_lits: "+strconv.FormatUint(uint64(before), 10)+"\n")
		return err
	})
}

func (s *satSimplifier) traceEliminationCollecting() {
	s.traceElimination("resolution_detail", "try_eliminate", 1354, func(w io.Writer) error {
		_, err := io.WriteString(w, "collecting number of after_clauses\n")
		return err
	})
}

func (s *satSimplifier) traceEliminationTooMany(after uint32) {
	s.traceElimination("resolution", "try_eliminate", 1369, func(w io.Writer) error {
		_, err := io.WriteString(w, "too many after clauses: "+strconv.FormatUint(uint64(after), 10)+"\n")
		return err
	})
}

func (s *satSimplifier) traceEliminationFound(before, after uint32) {
	s.traceElimination("resolution", "try_eliminate", 1375, func(w io.Writer) error {
		_, err := io.WriteString(w, "found var to eliminate, before: "+strconv.FormatUint(uint64(before), 10)+
			" after: "+strconv.FormatUint(uint64(after), 10)+"\n")
		return err
	})
}

func (s *satSimplifier) traceEliminationInputsAndResult(w io.Writer, first, second satClauseWrapper, trailingSpace bool) error {
	a, err := satTraceClauseWrapper(s.lifecycle.propagation.clauses, first)
	if err != nil {
		return err
	}
	b, err := satTraceClauseWrapper(s.lifecycle.propagation.clauses, second)
	if err != nil {
		return err
	}
	if _, err = io.WriteString(w, a+"\n"+b+"\n-->\n"); err != nil {
		return err
	}
	n, err := s.literalStore.Size(s.newCls)
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		l, err := s.literalStore.Read(s.newCls, i)
		if err != nil {
			return err
		}
		if i > 0 && !trailingSpace {
			if _, err = io.WriteString(w, " "); err != nil {
				return err
			}
		}
		if _, err = io.WriteString(w, l.String()); err != nil {
			return err
		}
		if trailingSpace {
			if _, err = io.WriteString(w, " "); err != nil {
				return err
			}
		}
	}
	_, err = io.WriteString(w, "\n")
	return err
}

func (s *satSimplifier) traceEliminationResolventDetail(first, second satClauseWrapper) {
	s.traceElimination("resolution_detail", "try_eliminate", 1365, func(w io.Writer) error {
		return s.traceEliminationInputsAndResult(w, first, second, true)
	})
}

func (s *satSimplifier) traceEliminationNewClause(first, second satClauseWrapper) {
	s.traceElimination("resolution_new_cls", "try_eliminate", 1401, func(w io.Writer) error {
		return s.traceEliminationInputsAndResult(w, first, second, false)
	})
}

func (s *satSimplifier) traceEliminatedClause(c StoredClauseRef) {
	s.traceElimination("resolution_bug", "remove_clauses", 1296, func(w io.Writer) error {
		clause, err := satTraceClause(s.lifecycle.propagation.clauses, c)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, "del_clause (elim_var): "+clause+"\n")
		return err
	})
}
