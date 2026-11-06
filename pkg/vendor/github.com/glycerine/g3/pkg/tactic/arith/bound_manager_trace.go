package arith

// Source: src/tactic/arith/bound_manager.cpp diagnostics and display.

import (
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (b *BoundManager) BindTrace(tags *diagnostic.TraceTags) error {
	if err := b.open(); err != nil {
		return err
	}
	b.trace = tags
	return nil
}
func (b *BoundManager) emit(function string, line int, body func(io.Writer) error) error {
	tags := b.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional("bound_manager", function, "src/tactic/arith/bound_manager.cpp", line, func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (b *BoundManager) writeExpr(out io.Writer, term ast.ASTHandle) (err error) {
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: b.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, stream.Close()) }()
	format, err := b.manager.SMT2PPExprTree(term, nil)
	if err != nil {
		return err
	}
	empty := b.context.EmptyValue()
	if err = b.manager.PrintFormat(stream, format, b.context, &empty, b.globals); err != nil {
		return err
	}
	text, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, text)
	return err
}
func (b *BoundManager) traceProcessing(term ast.ASTHandle) error {
	return b.emit("operator()", 92, func(out io.Writer) error {
		if _, err := io.WriteString(out, "processing:\n"); err != nil {
			return err
		}
		if err := b.writeExpr(out, term); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}
func (b *BoundManager) traceFound(term ast.ASTHandle) error {
	return b.emit("operator()", 127, func(out io.Writer) error {
		if _, err := io.WriteString(out, "found bound for:\n"); err != nil {
			return err
		}
		if err := b.writeExpr(out, term); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}
func (b *BoundManager) traceBounds(low, high *big.Rat) error {
	return b.emit("is_disjunctive_bound", 227, func(out io.Writer) error {
		_, err := fmt.Fprintf(out, "bounds: %s %s\n", low.RatString(), high.RatString())
		return err
	})
}
func (b *BoundManager) Display(out io.Writer) error {
	if err := b.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	for _, term := range b.variables {
		if bound, found := b.lowers[term]; found {
			op := "<="
			if bound.strict {
				op = "<"
			}
			if _, err := fmt.Fprintf(out, "%s %s", bound.value.RatString(), op); err != nil {
				return err
			}
		} else {
			if _, err := io.WriteString(out, "-oo <"); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(out, " "); err != nil {
			return err
		}
		if err := b.writeExpr(out, term); err != nil {
			return err
		}
		if _, err := io.WriteString(out, " "); err != nil {
			return err
		}
		if bound, found := b.uppers[term]; found {
			op := "<="
			if bound.strict {
				op = "<"
			}
			if _, err := fmt.Fprintf(out, "%s %s", op, bound.value.RatString()); err != nil {
				return err
			}
		} else {
			if _, err := io.WriteString(out, "< oo"); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}
