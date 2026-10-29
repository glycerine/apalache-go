package ast

// Source: src/ast/ast.cpp:2521; src/util/trace.h:40-46.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (m *ASTManager) distinctTraceTags() *diagnostic.TraceTags {
	if m.traceTags != nil {
		return m.traceTags
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (m *ASTManager) distinctTraceEnabled() bool {
	return m.distinctTraceTags().ShouldTrace("distinct")
}

func (m *ASTManager) traceDistinctExpanded(h ASTHandle) error {
	tags := m.distinctTraceTags()
	var bodyErr error
	_ = tags.EmitConditional("distinct", "mk_distinct_expanded", "src/ast/ast.cpp", 2521,
		func() bool { return true }, func(w io.Writer) (result error) {
			_, _ = io.WriteString(w, "expanded distinct:\n")
			g := uparams.BorrowProcessGlobalParamsService()
			if g == nil || g.Context == nil {
				bodyErr = uparams.ErrGlobalParamsOwner
				return bodyErr
			}
			format, err := m.SMT2PPExprTree(h, nil)
			if err != nil {
				bodyErr = err
				return err
			}
			out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.BudgetField})
			if err != nil {
				bodyErr = err
				return err
			}
			defer func() {
				bodyErr = errors.Join(bodyErr, out.Close())
				result = errors.Join(result, bodyErr)
			}()
			empty := g.Context.EmptyValue()
			if err = m.PrintFormat(out, format, g.Context, &empty, g); err != nil {
				bodyErr = err
				return err
			}
			text, err := out.String()
			if err != nil {
				bodyErr = err
				return err
			}
			_, _ = io.WriteString(w, text)
			_, _ = io.WriteString(w, "\n")
			return nil
		})
	return bodyErr
}
