package aig

// Source: src/tactic/aig/aig.cpp:435-443,1548-1559; src/util/trace.h:40-46.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func writeAIGRef(w io.Writer, l Literal) {
	if l.inverted {
		_, _ = io.WriteString(w, "-")
	}
	prefix, number := "#", l.node.id
	if l.node.and {
		prefix, number = "@", l.node.id-firstNodeID
	}
	_, _ = io.WriteString(w, prefix)
	_, _ = io.WriteString(w, strconv.FormatUint(uint64(number), 10))
}

func (m *Manager) smt2BooleanText(h ast.ASTHandle) (body string, result error) {
	g := uparams.BorrowProcessGlobalParamsService()
	if g == nil || g.Context == nil {
		return "", uparams.ErrGlobalParamsOwner
	}
	format, e := m.ast.SMT2PPBooleanTree(h, nil)
	if e != nil {
		return "", e
	}
	out, e := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.ast.BudgetField})
	if e != nil {
		return "", e
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	empty, e := g.Context.NewValue()
	if e != nil {
		return "", e
	}
	defer func() { result = errors.Join(result, g.Context.Del(&empty)) }()
	if e = m.ast.PrintFormat(out, format, g.Context, &empty, g); e != nil {
		return "", e
	}
	return out.String()
}

func (m *Manager) traceExprCacheResult(h ast.ASTHandle, l Literal) error {
	tags := m.ast.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("expr2aig") {
		return nil
	}
	return tags.EmitConditional("expr2aig", "cache_result", "src/tactic/aig/aig.cpp", 452,
		func() bool { return true }, func(w io.Writer) error {
			if _, e := io.WriteString(w, "caching:\n"); e != nil {
				return e
			}
			body, e := m.smt2BooleanText(h)
			if e != nil {
				return e
			}
			if _, e = io.WriteString(w, body+"\n---> "); e != nil {
				return e
			}
			writeAIGRef(w, l)
			_, e = io.WriteString(w, "\n")
			return e
		})
}

func (m *Manager) traceExprCacheDecRef(l Literal) {
	tags := m.ast.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("expr2aig") {
		return
	}
	_ = tags.EmitConditional("expr2aig", "~expr2aig", "src/tactic/aig/aig.cpp", 439,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dec-ref: ")
			writeAIGRef(w, l)
			_, _ = io.WriteString(w, " ref-count: ")
			_, _ = io.WriteString(w, strconv.FormatUint(uint64(l.node.refs), 10))
			_, _ = io.WriteString(w, "\n")
			return nil
		})
}
