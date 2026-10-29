package arith

// Source: src/tactic/arith/pb2bv_tactic.cpp diagnostics and live TRACE bodies.

import (
	"bytes"
	"fmt"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (t *PB2BVTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return t.bounds.BindTrace(tags)
}
func (t *PB2BVTactic) emit(tag, function string, line int, body func(io.Writer) error) error {
	tags := t.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional(tag, function, "src/tactic/arith/pb2bv_tactic.cpp", line, func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (t *PB2BVTactic) traceText(tag, function string, line int, body string) error {
	return t.emit(tag, function, line, func(out io.Writer) error { _, err := io.WriteString(out, body); return err })
}
func (t *PB2BVTactic) traceExpression(tag, function string, line int, prefix string, h ast.ASTHandle, suffix string) error {
	return t.emit(tag, function, line, func(out io.Writer) error {
		if _, err := io.WriteString(out, prefix); err != nil {
			return err
		}
		if err := t.bounds.writeExpr(out, h); err != nil {
			return err
		}
		_, err := io.WriteString(out, suffix)
		return err
	})
}
func (t *PB2BVTactic) tracePair(tag, function string, line int, first ast.ASTHandle, separator string, second ast.ASTHandle, suffix string) error {
	return t.emit(tag, function, line, func(out io.Writer) error {
		if err := t.bounds.writeExpr(out, first); err != nil {
			return err
		}
		if _, err := io.WriteString(out, separator); err != nil {
			return err
		}
		if err := t.bounds.writeExpr(out, second); err != nil {
			return err
		}
		_, err := io.WriteString(out, suffix)
		return err
	})
}
func (t *PB2BVTactic) displayPolynomial(out io.Writer, p pbPolynomial, c *big.Rat) error {
	for i, mon := range p {
		if i != 0 {
			if _, err := io.WriteString(out, " + "); err != nil {
				return err
			}
		}
		if mon.weight.Cmp(big.NewRat(1, 1)) != 0 {
			if _, err := io.WriteString(out, mon.weight.RatString()+"*"); err != nil {
				return err
			}
		}
		if mon.literal.negative {
			if _, err := io.WriteString(out, "~"); err != nil {
				return err
			}
		}
		if err := t.bounds.writeExpr(out, mon.literal.variable); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(out, " >= %s\n", c.RatString())
	return err
}
func (t *PB2BVTactic) tracePolynomial(tag, function string, line int, prefix string, p pbPolynomial, c *big.Rat, suffix string) error {
	return t.emit(tag, function, line, func(out io.Writer) error {
		if _, err := io.WriteString(out, prefix); err != nil {
			return err
		}
		if err := t.displayPolynomial(out, p, c); err != nil {
			return err
		}
		_, err := io.WriteString(out, suffix)
		return err
	})
}
func (t *PB2BVTactic) traceNumeral(tag, function string, line int, prefix string, c *big.Rat, suffix string) error {
	return t.emit(tag, function, line, func(out io.Writer) error { _, err := io.WriteString(out, prefix+c.RatString()+suffix); return err })
}
func (t *PB2BVTactic) traceBitWidth(bits uint32, sum *big.Rat, n int) error {
	return t.emit("num_bits_bug", "bitblast_pbc", 445, func(out io.Writer) error {
		_, err := fmt.Fprintf(out, "bits: %d sum: %s size: %d\n", bits, sum.RatString(), n)
		return err
	})
}
func (t *PB2BVTactic) traceSplit(p pbPolynomial, c *big.Rat, clause pbPolynomial) error {
	return t.emit("split_bug", "mk_pbc", 545, func(out io.Writer) error {
		if err := t.displayPolynomial(out, p, c); err != nil {
			return err
		}
		return t.displayPolynomial(out, clause, big.NewRat(1, 1))
	})
}
func (t *PB2BVTactic) traceEquality(term ast.ASTHandle, p pbPolynomial, c *big.Rat, p2 pbPolynomial, c2 *big.Rat, a, b ast.ASTHandle) error {
	return t.emit("pb2bv_convert", "convert", 824, func(out io.Writer) error {
		if err := t.bounds.writeExpr(out, term); err != nil {
			return err
		}
		if _, err := io.WriteString(out, "\n"); err != nil {
			return err
		}
		if err := t.displayPolynomial(out, p, c); err != nil {
			return err
		}
		if err := t.displayPolynomial(out, p2, c2); err != nil {
			return err
		}
		if _, err := io.WriteString(out, "--->\n"); err != nil {
			return err
		}
		if err := t.bounds.writeExpr(out, a); err != nil {
			return err
		}
		if _, err := io.WriteString(out, "\nAND\n"); err != nil {
			return err
		}
		if err := t.bounds.writeExpr(out, b); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}
func (t *PB2BVTactic) traceDiscard(term ast.ASTHandle, line int) error {
	return t.traceExpression("pb2bv", "convert", line, "discarding:\n", term, "\n")
}
func (t *PB2BVTactic) nonPBAt(term ast.ASTHandle, line int) error {
	if err := t.traceExpression("pb2bv", "throw_non_pb", line, "Not pseudo-Boolean: ", term, "\n"); err != nil {
		return err
	}
	return pbNonPB{term}
}
func (t *PB2BVTactic) nonPB(term ast.ASTHandle) error { return t.nonPBAt(term, 578) }
func (t *PB2BVTactic) unsupported(term ast.ASTHandle) error {
	var out bytes.Buffer
	if err := t.bounds.writeExpr(&out, term); err != nil {
		return err
	}
	return tactic.NewTacticException("goal is in a fragment unsupported by pb2bv. Offending expression: " + out.String())
}
