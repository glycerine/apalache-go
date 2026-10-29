package bv

// Source: src/tactic/bv/bv_size_reduction_tactic.cpp live signed-bound traces.

import (
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (t *BVSizeReductionTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	if t.current.filter != nil {
		return t.current.filter.BindTrace(tags, t.globals)
	}
	return nil
}
func (t *BVSizeReductionTactic) emit(tag string, line int, body func(io.Writer) error) error {
	tags := t.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	function := "operator()"
	if line == 127 || line == 142 {
		function = "collect_bounds"
	}
	err := tags.EmitConditional(tag, function, "src/tactic/bv/bv_size_reduction_tactic.cpp", line, func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (t *BVSizeReductionTactic) writeSizeExpr(w io.Writer, h ast.ASTHandle) (err error) {
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: t.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, stream.Close()) }()
	format, err := t.manager.SMT2PPExprTree(h, nil)
	if err != nil {
		return err
	}
	fm, err := t.manager.GetFormatManager()
	if err != nil {
		return err
	}
	held, err := fm.NewRef(format)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, fm.DelRef(&held)) }()
	empty := t.context.EmptyValue()
	if err = t.manager.PrintFormat(stream, format, t.context, &empty, t.globals); err != nil {
		return err
	}
	text, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, text)
	return err
}
func (t *BVSizeReductionTactic) writeSizeName(w io.Writer, v ast.ASTHandle) error {
	d, err := t.manager.GetDecl(v)
	if err != nil {
		return err
	}
	name, err := t.manager.GetASTName(d)
	if err != nil {
		return err
	}
	return t.manager.Symbols.WriteDisplay(w, name)
}
func (t *BVSizeReductionTactic) traceCollected(f ast.ASTHandle, negated, lower bool) error {
	line := 127
	if lower {
		line = 142
	}
	return t.emit("bv_size_reduction", line, func(w io.Writer) error {
		if negated {
			if _, err := io.WriteString(w, "not "); err != nil {
				return err
			}
		}
		if err := t.writeSizeExpr(w, f); err != nil {
			return err
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}
func (t *BVSizeReductionTactic) traceBounds(r *bvSizeReductionImplementation) error {
	return t.emit("bv_size_reduction", 198, func(w io.Writer) error {
		if _, err := io.WriteString(w, "m_signed_lowers: \n"); err != nil {
			return err
		}
		for v, bound := range r.lower {
			if err := t.writeSizeExpr(w, v); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, " >= %s\n", bound.String()); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, "m_signed_uppers: \n"); err != nil {
			return err
		}
		for v, bound := range r.upper {
			if err := t.writeSizeExpr(w, v); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, " <= %s\n", bound.String()); err != nil {
				return err
			}
		}
		return nil
	})
}
func (t *BVSizeReductionTactic) traceInterval(v ast.ASTHandle, l, u *big.Int) error {
	return t.emit("bv_size_reduction", 216, func(w io.Writer) error {
		if _, err := fmt.Fprintf(w, "%s <= ", l.String()); err != nil {
			return err
		}
		if err := t.writeSizeName(w, v); err != nil {
			return err
		}
		_, err := fmt.Fprintf(w, " <= %s\n", u.String())
		return err
	})
}
func (t *BVSizeReductionTactic) traceWidth(v ast.ASTHandle, l, u *big.Int, bits uint32, mode int) error {
	line := 257
	if mode < 0 {
		line = 236
	} else if mode > 0 {
		line = 246
	}
	return t.emit("bv_size_reduction", line, func(w io.Writer) error {
		if mode < 0 {
			if _, err := io.WriteString(w, " l <= "); err != nil {
				return err
			}
			if err := t.writeSizeName(w, v); err != nil {
				return err
			}
			_, err := fmt.Fprintf(w, " <= u <= 0  --> %d bits\n", bits)
			return err
		}
		if mode > 0 {
			if _, err := io.WriteString(w, " l <= "); err != nil {
				return err
			}
			if err := t.writeSizeName(w, v); err != nil {
				return err
			}
			_, err := fmt.Fprintf(w, " <= 0 <= u  --> %d bits\n", bits)
			return err
		}
		if _, err := fmt.Fprintf(w, "%s <= ", l.String()); err != nil {
			return err
		}
		if err := t.writeSizeName(w, v); err != nil {
			return err
		}
		_, err := fmt.Fprintf(w, " <= %s --> %d bits\n", u.String(), bits)
		return err
	})
}
