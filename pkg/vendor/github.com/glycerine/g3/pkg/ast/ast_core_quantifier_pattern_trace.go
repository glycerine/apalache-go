package ast

// Source: src/ast/ast.cpp:2356-2361; src/ast/ast_smt2_pp.cpp:760-761.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (m *ASTManager) traceQuantifierPattern(i int, pattern ASTHandle) error {
	tags := m.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("ast") {
		return nil
	}
	return tags.EmitConditional("ast", "mk_quantifier", "src/ast/ast.cpp", 2358,
		func() bool { return true }, func(w io.Writer) (result error) {
			if _, err := io.WriteString(w, strconv.Itoa(i)+" "); err != nil {
				return err
			}
			g := uparams.BorrowProcessGlobalParamsService()
			if g == nil || g.Context == nil {
				return uparams.ErrGlobalParamsOwner
			}
			format, err := m.SMT2PPExprTree(pattern, nil)
			if err != nil {
				return err
			}
			out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.BudgetField})
			if err != nil {
				return err
			}
			defer func() { result = errors.Join(result, out.Close()) }()
			empty := g.Context.EmptyValue()
			if err = m.PrintFormat(out, format, g.Context, &empty, g); err != nil {
				return err
			}
			body, err := out.String()
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, body+"\n")
			return err
		})
}
