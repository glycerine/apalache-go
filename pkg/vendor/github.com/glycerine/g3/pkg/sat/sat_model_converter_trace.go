package sat

// Source: src/sat/sat_model_converter.cpp:108,142,152,163,181; retained util/trace sites.

import (
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (m *satModelConverter) trace(tag, function string, line int, condition func() bool, body func(io.Writer) error) {
	tags := m.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional(tag, function, "src/sat/sat_model_converter.cpp", line, condition, body)
}

func (m *satModelConverter) traceFailedClause(lits []Literal) {
	m.trace("sat_model_bug", "check_model", 108, func() bool { return true }, func(w io.Writer) error {
		var body strings.Builder
		body.WriteString("failed eliminated: ")
		for i, lit := range lits {
			if i != 0 {
				body.WriteByte(' ')
			}
			body.WriteString(lit.String())
		}
		body.WriteByte('\n')
		_, err := io.WriteString(w, body.String())
		return err
	})
}

func (m *satModelConverter) traceInsertClause(a *ClauseAllocator, c StoredClauseRef) {
	m.trace("sat_mc_bug", "insert", 142, func() bool { return true }, func(w io.Writer) error {
		body, err := satTraceClause(a, c)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, "adding: "+body+"\n")
		return err
	})
}

func (m *satModelConverter) traceInsertBinary(a, b Literal) {
	m.trace("sat_mc_bug", "insert", 152, func() bool { return true }, func(w io.Writer) error {
		_, err := io.WriteString(w, "adding (binary): "+a.String()+" "+b.String()+"\n")
		return err
	})
}

func (m *satModelConverter) traceInsertWrapper(a *ClauseAllocator, c satClauseWrapper) {
	m.trace("sat_mc_bug", "insert", 163, func() bool { return true }, func(w io.Writer) error {
		n, err := c.Size(a)
		if err != nil {
			return err
		}
		var body strings.Builder
		body.WriteString("adding (wrapper): ")
		for i := uint32(0); i < n; i++ {
			lit, err := c.Literal(a, i)
			if err != nil {
				return err
			}
			body.WriteString(lit.String())
			body.WriteByte(' ')
		}
		body.WriteByte('\n')
		_, err = io.WriteString(w, body.String())
		return err
	})
}

func (m *satModelConverter) traceInvariantVar(v BoolVar) {
	m.trace("sat_model_converter", "check_invariant", 181, func() bool { return true }, func(w io.Writer) (result error) {
		if _, err := io.WriteString(w, "var: "+strconv.FormatUint(uint64(v), 10)+"\n"); err != nil {
			return err
		}
		out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.budget})
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, out.Close()) }()
		if err = m.DisplayInto(out); err != nil {
			return err
		}
		body, err := out.String()
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, body)
		return err
	})
}
