package smtcore

// Source: src/smt/theory_bv.cpp:436-439,557,574,611-615,662-679,
// 859,1052,1087-1088,1285,1325.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTBitVectorTheory) traceBVExpression(tag, function string, line int, prefix string, expression ast.ASTHandle) error {
	return t.emitBVTrace(tag, function, line, func(out io.Writer) error {
		if _, err := io.WriteString(out, prefix); err != nil {
			return err
		}
		if err := t.displayBVTraceExpression(out, expression); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}

func (t *SMTBitVectorTheory) traceBVFixedAxiom(left, right ast.ASTHandle) error {
	return t.emitBVTrace("bv", "add_fixed_eq", 436, func(out io.Writer) error {
		state, err := t.State()
		if err != nil {
			return err
		}
		if err = t.displayBVTraceExpression(out, left); err != nil {
			return err
		}
		if _, err = io.WriteString(out, " = "); err != nil {
			return err
		}
		if err = t.displayBVTraceExpression(out, right); err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, " %d\n", state.scopeLevel)
		return err
	})
}

func (t *SMTBitVectorTheory) traceBVBVToIntAxiom(expression ast.ASTHandle) error {
	return t.traceBVExpression("bv2int_bug", "assert_bv2int_axiom", 574, "bv2int:\n", expression)
}

func (t *SMTBitVectorTheory) traceBVBVToIntEquality(left, right ast.ASTHandle, literal SMTLiteral) error {
	return t.emitBVTrace("bv", "assert_bv2int_axiom", 611, func(out io.Writer) error {
		state, err := t.State()
		if err != nil {
			return err
		}
		if err = t.displayBVTraceExpression(out, left); err != nil {
			return err
		}
		if _, err = io.WriteString(out, "\n"); err != nil {
			return err
		}
		if err = t.displayBVTraceExpression(out, right); err != nil {
			return err
		}
		if _, err = io.WriteString(out, "\n"); err != nil {
			return err
		}
		if err = literal.Display(out, state.manager, state.boolExpr); err != nil {
			return err
		}
		_, err = io.WriteString(out, "\n")
		return err
	})
}

func (t *SMTBitVectorTheory) traceBVIntToBVMainEquality(left, right ast.ASTHandle) error {
	return t.emitBVTrace("bv", "assert_int2bv_axiom", 662, func(out io.Writer) error {
		if err := t.displayBVTraceExpression(out, left); err != nil {
			return err
		}
		if _, err := io.WriteString(out, " == \n"); err != nil {
			return err
		}
		if err := t.displayBVTraceExpression(out, right); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}

func (t *SMTBitVectorTheory) traceBVIntToBVBitEquality(left, right ast.ASTHandle) error {
	return t.emitBVTrace("bv", "assert_int2bv_axiom", 678, func(out io.Writer) error {
		if err := t.displayBVTraceExpression(out, left); err != nil {
			return err
		}
		if _, err := io.WriteString(out, " == "); err != nil {
			return err
		}
		if err := t.displayBVTraceExpression(out, right); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}

func (t *SMTBitVectorTheory) traceBVScopePop(count uint32) error {
	return t.emitBVTrace("bv", "pop_scope_eh", 1325, func(out io.Writer) error {
		_, err := fmt.Fprintf(out, "%d\n", count)
		return err
	})
}
