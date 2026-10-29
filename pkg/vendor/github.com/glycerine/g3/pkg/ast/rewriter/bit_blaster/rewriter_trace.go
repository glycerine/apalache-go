package bit_blaster

// Source: src/ast/rewriter/bit_blaster/bit_blaster_rewriter.cpp:350-352,519-520.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (r *BitBlasterRewriter) BindTrace(tags *diagnostic.TraceTags) error {
	if err := r.open(); err != nil {
		return err
	}
	r.trace = tags
	return nil
}
func (r *BitBlasterRewriter) traceApplication(decl ast.ASTHandle, args ast.AppArguments, unsupported bool) error {
	tags := r.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	line := 350
	if unsupported {
		line = 519
	}
	err := tags.EmitConditional("bit_blaster", "reduce_app", "src/ast/rewriter/bit_blaster/bit_blaster_rewriter.cpp", line, func() bool { return true }, func(w io.Writer) error {
		sym, e := r.manager.GetASTName(decl)
		if e != nil {
			return e
		}
		name, e := r.manager.Symbols.String(sym)
		if e != nil {
			return e
		}
		if unsupported {
			if _, e = io.WriteString(w, "non-supported operator: "+name+"\n"); e != nil {
				return e
			}
		} else {
			if _, e = io.WriteString(w, name+" "); e != nil {
				return e
			}
		}
		for i := uint32(0); i < args.Count(); i++ {
			h, e := args.At(i)
			if e != nil {
				return e
			}
			if e = r.writeTraceExpr(w, h); e != nil {
				return e
			}
			separator := " "
			if unsupported {
				separator = "\n"
			}
			if _, e = io.WriteString(w, separator); e != nil {
				return e
			}
		}
		if !unsupported {
			_, e = io.WriteString(w, "\n")
		}
		return e
	})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (r *BitBlasterRewriter) writeTraceExpr(w io.Writer, h ast.ASTHandle) (err error) {
	g := uparams.BorrowProcessGlobalParamsService()
	if g == nil {
		return uparams.ErrGlobalParamsOwner
	}
	f, err := r.manager.SMT2PPExprTree(h, nil)
	if err != nil {
		return err
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: r.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, stream.Close()) }()
	p := g.Context.EmptyValue()
	if err = r.manager.PrintFormat(stream, f, g.Context, &p, g); err != nil {
		return err
	}
	s, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, s)
	return err
}
