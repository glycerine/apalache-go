package smtcore

// Source: src/smt/theory_bv.cpp:321,337-350,471-473.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (t *SMTBitVectorTheory) emitBVTrace(tag, function string, line int, body func(io.Writer) error) error {
	tags := t.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if tags == nil || !tags.ShouldTrace(tag) {
		return nil
	}
	var operandErr error
	err := tags.EmitConditional(tag, function, "src/smt/theory_bv.cpp", line, func() bool { return true }, func(out io.Writer) error {
		operandErr = body(assertedTraceWriter{out})
		return operandErr
	})
	if operandErr != nil {
		return operandErr
	}
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (t *SMTBitVectorTheory) displayBVTraceExpression(out io.Writer, expression ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	printer, err := ast.NewSimpleASTPrinterContext(state.manager, out, out)
	if err != nil {
		return err
	}
	return printer.DisplayExpr(out, expression, 0)
}

func (t *SMTBitVectorTheory) traceBVInitBit(owner ast.ASTHandle, index uint32, bit SMTLiteral) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	id, err := state.manager.GetASTID(owner)
	if err != nil {
		return err
	}
	expression, err := state.literalExpression(bit)
	if err != nil {
		return err
	}
	return t.emitBVTrace("init_bits", "init_bits", 321, func(out io.Writer) error {
		if _, err = fmt.Fprintf(out, "bit %d of #%d\n", index, id); err != nil {
			return err
		}
		if err = t.displayBVTraceExpression(out, expression); err != nil {
			return err
		}
		_, err = io.WriteString(out, "\n")
		return err
	})
}

func (t *SMTBitVectorTheory) traceBVWatchedLiteral(bit SMTLiteral) error {
	return t.emitBVTrace("find_wpos", "find_wpos", 337, func(out io.Writer) error {
		if _, err := io.WriteString(out, "curr bit: "); err != nil {
			return err
		}
		if err := smtPBWriteLiteral(out, bit); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}

func (t *SMTBitVectorTheory) traceBVMovedWatch(variable SMTTheoryVar, position uint32, line int) error {
	return t.emitBVTrace("find_wpos", "find_wpos", line, func(out io.Writer) error {
		_, err := fmt.Fprintf(out, "moved wpos of v%d to %d\n", variable, position)
		return err
	})
}

func (t *SMTBitVectorTheory) traceBVFixedWatch(variable SMTTheoryVar) error {
	return t.emitBVTrace("find_wpos", "find_wpos", 350, func(out io.Writer) error {
		_, err := fmt.Fprintf(out, "v%d is a fixed variable.\n", variable)
		return err
	})
}

func (t *SMTBitVectorTheory) traceBVFixedEquality(first, second SMTTheoryVar) error {
	return t.emitBVTrace("fixed_var_eh", "fixed_var_eh", 471, func(out io.Writer) error {
		if _, err := fmt.Fprintf(out, "detected equality: v%d = v%d\n", first, second); err != nil {
			return err
		}
		if err := t.DisplayVariable(out, first); err != nil {
			return err
		}
		return t.DisplayVariable(out, second)
	})
}
