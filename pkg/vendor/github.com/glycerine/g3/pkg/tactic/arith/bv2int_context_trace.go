package arith

// Source: src/tactic/arith/bv2int_rewriter.cpp:86.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (r *BV2IntContext) BindTrace(tags *diagnostic.TraceTags) error {
	if err := r.open(); err != nil {
		return err
	}
	r.trace = tags
	return nil
}
func (r *BV2IntContext) writeExpr(out io.Writer, h ast.ASTHandle) (err error) {
	g := uparams.BorrowProcessGlobalParamsService()
	if g == nil || g.Context != r.context {
		return uparams.ErrParamsOwner
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: r.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, stream.Close()) }()
	format, err := r.manager.SMT2PPExprTree(h, nil)
	if err != nil {
		return err
	}
	empty := r.context.EmptyValue()
	if err = r.manager.PrintFormat(stream, format, r.context, &empty, g); err != nil {
		return err
	}
	text, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, text)
	return err
}
func (r *BV2IntContext) tracePower(v, logx ast.ASTHandle) error {
	tags := r.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional("bv2int_rewriter", "collect_power2", "src/tactic/arith/bv2int_rewriter.cpp", 86, func() bool { return true }, func(out io.Writer) error {
		if err := r.writeExpr(out, v); err != nil {
			return err
		}
		if _, err := io.WriteString(out, " |-> "); err != nil {
			return err
		}
		if err := r.writeExpr(out, logx); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
