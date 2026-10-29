package ast

// Source: src/ast/well_sorted.cpp:49; src/util/trace.h:40-46.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (m *ASTManager) wellSortedTraceParams(c *uparams.ParamsContext, p *uparams.ParamsRef, g *uparams.GlobalParamsService) (*uparams.ParamsContext, *uparams.ParamsRef, *uparams.GlobalParamsService, error) {
	if c != nil && p != nil && g != nil {
		return c, p, g, nil
	}
	g = uparams.BorrowProcessGlobalParamsService()
	if g == nil || g.Context == nil || g.CheckOwner(g.Context, m.BudgetField) != nil || g.Context.CheckServices(m.Symbols, m.numerals) != nil {
		return nil, nil, nil, nil
	}
	c = g.Context
	empty := c.EmptyValue()
	return c, &empty, g, nil
}

func (m *ASTManager) wellSortedTraceFormat(format ASTHandle, c *uparams.ParamsContext, p *uparams.ParamsRef, g *uparams.GlobalParamsService) (body string, result error) {
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.BudgetField})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	if err = m.PrintFormat(out, format, c, p, g); err != nil {
		return "", err
	}
	return out.String()
}

func (m *ASTManager) wellSortedTraceSMT2(app ASTHandle, c *uparams.ParamsContext, p *uparams.ParamsRef, g *uparams.GlobalParamsService) (string, error) {
	format, err := m.SMT2PPExprTree(app, nil)
	if err != nil {
		return "", err
	}
	return m.wellSortedTraceFormat(format, c, p, g)
}

func (m *ASTManager) traceWellSortedArity(app ASTHandle, c *uparams.ParamsContext, p *uparams.ParamsRef, g *uparams.GlobalParamsService) error {
	tags := m.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("ws") {
		return nil
	}
	return tags.EmitConditional("ws", "operator()", "src/ast/well_sorted.cpp", 49,
		func() bool { return true }, func(w io.Writer) error {
			if _, err := io.WriteString(w, "unexpected number of arguments.\n"); err != nil {
				return err
			}
			c, p, g, err := m.wellSortedTraceParams(c, p, g)
			if err != nil {
				return err
			}
			body, err := m.wellSortedTraceSMT2(app, c, p, g)
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, body)
			return err
		})
}

func (m *ASTManager) traceWellSortedMismatch(info wellSortedWarning, c *uparams.ParamsContext, p *uparams.ParamsRef, g *uparams.GlobalParamsService) error {
	tags := m.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("tc") {
		return nil
	}
	return tags.EmitConditional("tc", "operator()", "src/ast/well_sorted.cpp", 59,
		func() bool { return true }, func(w io.Writer) error {
			c, p, g, err := m.wellSortedTraceParams(c, p, g)
			if err != nil {
				return err
			}
			write := func(s string) error { _, err := io.WriteString(w, s); return err }
			if err = write("sort mismatch on argument #" + strconv.FormatUint(uint64(info.argument-1), 10) + ".\n"); err != nil {
				return err
			}
			body, err := m.wellSortedTraceSMT2(info.app, c, p, g)
			if err != nil {
				return err
			}
			if err = write(body + "Sort mismatch for argument " + strconv.FormatUint(uint64(info.argument), 10) + " of "); err != nil {
				return err
			}
			body, err = m.wellSortedTraceSMT2(info.app, c, p, g)
			if err != nil {
				return err
			}
			if err = write(body + "\n"); err != nil {
				return err
			}
			for _, item := range []struct {
				prefix string
				handle ASTHandle
				decl   bool
				end    string
			}{
				{"Expected sort: ", info.want, false, "\n"},
				{"Actual sort:   ", info.actual, false, "\n"},
				{"Function sort: ", info.decl, true, "."},
			} {
				if err = write(item.prefix); err != nil {
					return err
				}
				var format ASTHandle
				if item.decl {
					format, err = m.SMT2PPDeclareFun(item.handle)
				} else {
					format, err = m.SMT2PPSort(item.handle)
				}
				if err != nil {
					return err
				}
				body, err = m.wellSortedTraceFormat(format, c, p, g)
				if err != nil {
					return err
				}
				if err = write(body + item.end); err != nil {
					return err
				}
			}
			return nil
		})
}
