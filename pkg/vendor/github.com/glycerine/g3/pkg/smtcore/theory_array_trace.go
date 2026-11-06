package smtcore

// Source: src/smt/theory_array_base.cpp:944-957.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func arraySourceAssert(line int32, condition string, evaluate func() bool) error {
	if !diagnostic.DebugBuild {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !enabled {
		return err
	}
	if !evaluate() {
		return diagnostic.DebugAssertionFailure(d, "src/smt/theory_array.cpp", line, condition)
	}
	return nil
}

func arraySourceVerify(line int32, condition string, evaluate func() bool) error {
	if evaluate() {
		return nil
	}
	err := diagnostic.BorrowProcessDebugContext().VerifyFailure("src/smt/theory_array.cpp", line, condition)
	if err != nil {
		return err
	}
	return ast.ErrASTDomain
}

func (t *SMTArrayTheory) emitArrayTrace(tag, function string, line int, condition func() bool, body func(io.Writer) error) error {
	return t.emitArrayTraceAt("src/smt/theory_array_base.cpp", tag, function, line, condition, body)
}

func (t *SMTArrayTheory) emitFullArrayTrace(tag, function string, line int, condition func() bool, body func(io.Writer) error) error {
	return t.emitArrayTraceAt("src/smt/theory_array_full.cpp", tag, function, line, condition, body)
}

func (t *SMTArrayTheory) emitSimpleArrayTrace(tag, function string, line int, condition func() bool, body func(io.Writer) error) error {
	return t.emitArrayTraceAt("src/smt/theory_array.cpp", tag, function, line, condition, body)
}

func (t *SMTArrayTheory) emitArrayTraceAt(file, tag, function string, line int, condition func() bool, body func(io.Writer) error) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	tags := state.manager.BorrowSourceTraceTags()
	if tags == nil || !tags.ShouldTrace(tag) {
		return nil
	}
	if condition == nil {
		condition = func() bool { return true }
	}
	var operandErr error
	err = tags.EmitConditional(tag, function, file, line, condition, func(out io.Writer) error {
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

func (t *SMTArrayTheory) traceFullArrayExpression(function string, line int, expression ast.ASTHandle) error {
	return t.emitFullArrayTrace("array", function, line, nil, func(out io.Writer) error {
		state, err := t.State()
		if err != nil {
			return err
		}
		if err = displayArrayTraceExpression(state.manager, out, expression); err != nil {
			return err
		}
		_, err = io.WriteString(out, "\n")
		return err
	})
}

func (t *SMTArrayTheory) traceSimpleArrayExpression(tag, function string, line int, prefix string, expression ast.ASTHandle) error {
	return t.emitSimpleArrayTrace(tag, function, line, nil, func(out io.Writer) error {
		if _, err := io.WriteString(out, prefix); err != nil {
			return err
		}
		state, err := t.State()
		if err != nil {
			return err
		}
		if err = displayArrayTraceExpression(state.manager, out, expression); err != nil {
			return err
		}
		_, err = io.WriteString(out, "\n")
		return err
	})
}

func (t *SMTArrayTheory) traceArrayExpressions(tag, function string, line int, prefix string, expressions ...ast.ASTHandle) error {
	return t.emitArrayTrace(tag, function, line, nil, func(out io.Writer) error {
		if _, err := io.WriteString(out, prefix); err != nil {
			return err
		}
		state, err := t.State()
		if err != nil {
			return err
		}
		for index, expression := range expressions {
			if index != 0 {
				if _, err = io.WriteString(out, " "); err != nil {
					return err
				}
			}
			if err = displayArrayTraceExpression(state.manager, out, expression); err != nil {
				return err
			}
		}
		_, err = io.WriteString(out, "\n")
		return err
	})
}

func (t *SMTArrayTheory) makeArraySelect(arguments []ast.ASTHandle) (ast.ASTHandle, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := state.manager.MkFamilyApp(t.FamilyID(), ast.ASTArrayOpSelect, nil, arguments)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	err = t.emitArrayTrace("mk_var_bug", "mk_select", 45, nil, func(out io.Writer) error {
		id, err := state.manager.GetASTID(result)
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintf(out, "mk_select: %d num_args: %d", id, len(arguments)); err != nil {
			return err
		}
		for _, argument := range arguments {
			id, err = state.manager.GetASTID(argument)
			if err != nil {
				return err
			}
			if _, err = fmt.Fprintf(out, " %d", id); err != nil {
				return err
			}
		}
		_, err = io.WriteString(out, "\n")
		return err
	})
	return result, err
}

func (t *SMTArrayTheory) foundUnsupportedArrayOperation(expression ast.ASTHandle) error {
	if err := t.traceArrayExpressions("array", "found_unsupported_op", 36, "", expression); err != nil {
		return err
	}
	t.unsupported = true
	return nil
}

func (t *SMTArrayTheory) assertArrayAxiom(literals []SMTLiteral) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	if err = t.emitArrayTrace("array_axiom", "assert_axiom", 70, nil, func(out io.Writer) error {
		if _, err := io.WriteString(out, "literals:\n"); err != nil {
			return err
		}
		for _, literal := range literals {
			if literal.Sign() {
				if _, err := io.WriteString(out, "not "); err != nil {
					return err
				}
			}
			expression, err := smtLiteralExpression(literal, state.boolExpr)
			if err != nil {
				return err
			}
			if err = displayArrayTraceExpression(state.manager, out, expression); err != nil {
				return err
			}
			if _, err = io.WriteString(out, " \n"); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return state.addTheoryAxiom(t.FamilyID(), literals)
}

func (t *SMTArrayTheory) traceArrayModelValue(generator *SMTModelGenerator, node *SMTEnode, reads []*SMTEnode, fallback *SMTEnode) error {
	if generator == nil || node == nil || node.root == nil {
		return ast.ErrASTDomain
	}
	tags := generator.traceTags()
	err := tags.EmitConditional("array", "mk_value", "src/smt/theory_array_base.cpp", 944, func() bool { return true }, func(raw io.Writer) error {
		out := assertedTraceWriter{raw}
		if err := displayArrayTraceExpression(generator.manager, out, node.root.owner); err != nil {
			return err
		}
		if _, err := io.WriteString(out, "\n"); err != nil {
			return err
		}
		for _, read := range reads {
			if read == nil || read.root == nil {
				return ast.ErrASTDomain
			}
			id, err := read.root.OwnerID()
			if err != nil {
				return err
			}
			if _, err = fmt.Fprintf(out, "#%d ", id); err != nil {
				return err
			}
			if err = displayArrayTraceExpression(generator.manager, out, read.owner); err != nil {
				return err
			}
			if _, err = io.WriteString(out, "\n"); err != nil {
				return err
			}
		}
		if fallback != nil {
			if _, err := io.WriteString(out, "else: "); err != nil {
				return err
			}
			if err := displayArrayTraceExpression(generator.manager, out, fallback.owner); err != nil {
				return err
			}
			_, err := io.WriteString(out, "\n")
			return err
		}
		return nil
	})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func displayArrayTraceExpression(manager *ast.ASTManager, out io.Writer, expression ast.ASTHandle) error {
	printer, err := ast.NewSimpleASTPrinterContext(manager, out, out)
	if err != nil {
		return err
	}
	return printer.DisplayExpr(out, expression, 0)
}
