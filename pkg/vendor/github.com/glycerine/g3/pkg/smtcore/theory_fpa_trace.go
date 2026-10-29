package smtcore

// Source: retained TRACE/CTRACE sites in src/smt/theory_fpa.cpp.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (t *SMTFloatingPointTheory) fpaTraceTags() (*diagnostic.TraceTags, error) {
	state, err := t.State()
	if err != nil {
		return nil, err
	}
	return state.manager.BorrowSourceTraceTags(), nil
}

func (t *SMTFloatingPointTheory) emitFPATrace(tag, function string, line int, condition func() bool, body func(io.Writer) error) error {
	tags, err := t.fpaTraceTags()
	if err != nil {
		return err
	}
	return t.emitFPATraceWithTags(tags, tag, function, line, condition, body)
}

func (t *SMTFloatingPointTheory) emitFPATraceWithTags(tags *diagnostic.TraceTags, tag, function string, line int, condition func() bool, body func(io.Writer) error) error {
	if tags == nil || !tags.ShouldTrace(tag) {
		return nil
	}
	if condition == nil {
		condition = func() bool { return true }
	}
	var operandErr error
	err := tags.EmitConditional(tag, function, "src/smt/theory_fpa.cpp", line, condition, func(out io.Writer) error {
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

func (t *SMTFloatingPointTheory) emitFPAModelTrace(generator *SMTModelGenerator, tag, function string, line int, condition func() bool, body func(io.Writer) error) error {
	if generator != nil && generator.trace != nil {
		return t.emitFPATraceWithTags(generator.trace, tag, function, line, condition, body)
	}
	return t.emitFPATrace(tag, function, line, condition, body)
}

func (t *SMTFloatingPointTheory) displayFPATraceExpression(out io.Writer, expression ast.ASTHandle) error {
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

func (t *SMTFloatingPointTheory) displayFPATraceSort(out io.Writer, sort ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	printer, err := ast.NewSimpleASTPrinterContext(state.manager, out, out)
	if err != nil {
		return err
	}
	return printer.DisplaySort(out, sort, 0)
}

func (t *SMTFloatingPointTheory) traceFPAExpression(tag, function string, line int, prefix string, expression ast.ASTHandle) error {
	return t.emitFPATrace(tag, function, line, nil, func(out io.Writer) error {
		if _, err := io.WriteString(out, prefix); err != nil {
			return err
		}
		if err := t.displayFPATraceExpression(out, expression); err != nil {
			return err
		}
		return diagnostic.WriteTraceEndl(out)
	})
}

func (t *SMTFloatingPointTheory) traceFPAConversion(line int, heading string, input, output ast.ASTHandle) error {
	return t.emitFPATrace("t_fpa_detail", "convert", line, nil, func(out io.Writer) error {
		if _, err := io.WriteString(out, heading+"\n"); err != nil {
			return err
		}
		if err := t.displayFPATraceExpression(out, input); err != nil {
			return err
		}
		if _, err := io.WriteString(out, "\n -> \n"); err != nil {
			return err
		}
		if err := t.displayFPATraceExpression(out, output); err != nil {
			return err
		}
		return diagnostic.WriteTraceEndl(out)
	})
}

func (t *SMTFloatingPointTheory) traceFPAConvertAtom(expression ast.ASTHandle) error {
	return t.traceFPAExpression("t_fpa_detail", "convert_atom", 297, "converting atom: ", expression)
}

func (t *SMTFloatingPointTheory) traceFPAConvertTerm(input, output ast.ASTHandle) error {
	return t.emitFPATrace("t_fpa_detail", "convert_term", 316, nil, func(out io.Writer) error {
		if _, err := io.WriteString(out, "term: "); err != nil {
			return err
		}
		if err := t.displayFPATraceExpression(out, input); err != nil {
			return err
		}
		if _, err := io.WriteString(out, "\nconverted term: "); err != nil {
			return err
		}
		if err := t.displayFPATraceExpression(out, output); err != nil {
			return err
		}
		return diagnostic.WriteTraceEndl(out)
	})
}

func (t *SMTFloatingPointTheory) traceFPASideCondition(expression ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	return t.emitFPATrace("t_fpa", "mk_side_conditions", 404, func() bool {
		return !state.manager.IsTrue(expression)
	}, func(out io.Writer) error {
		if _, err = io.WriteString(out, "side condition: "); err != nil {
			return err
		}
		if err = t.displayFPATraceExpression(out, expression); err != nil {
			return err
		}
		return diagnostic.WriteTraceEndl(out)
	})
}

func (t *SMTFloatingPointTheory) traceFPAWrapUnwrap(expression ast.ASTHandle) error {
	return t.emitFPATrace("t_fpa", "relevant_eh", 679, nil, func(out io.Writer) error {
		if _, err := io.WriteString(out, "w/u eq: \n"); err != nil {
			return err
		}
		if err := t.displayFPATraceExpression(out, expression); err != nil {
			return err
		}
		return diagnostic.WriteTraceEndl(out)
	})
}

func (t *SMTFloatingPointTheory) traceFPAEqualityCallback(function string, line int, symbol string, left, right SMTTheoryVar) error {
	leftNode, err := t.Enode(left)
	if err != nil {
		return err
	}
	rightNode, err := t.Enode(right)
	if err != nil {
		return err
	}
	label := "new eq"
	if symbol == "!=" {
		label = "new diseq"
	}
	return t.emitFPATrace("t_fpa", function, line, nil, func(out io.Writer) error {
		if _, err = fmt.Fprintf(out, "%s: %d %s %d\n", label, left, symbol, right); err != nil {
			return err
		}
		if err = t.displayFPATraceExpression(out, leftNode.Owner()); err != nil {
			return err
		}
		if _, err = fmt.Fprintf(out, "\n %s \n", symbol); err != nil {
			return err
		}
		if err = t.displayFPATraceExpression(out, rightNode.Owner()); err != nil {
			return err
		}
		return diagnostic.WriteTraceEndl(out)
	})
}

func (t *SMTFloatingPointTheory) traceFPAConvertedEquality(left, right ast.ASTHandle) error {
	return t.emitFPATrace("t_fpa_detail", "new_eq_eh", 545, nil, func(out io.Writer) error {
		if _, err := io.WriteString(out, "xc = "); err != nil {
			return err
		}
		if err := t.displayFPATraceExpression(out, left); err != nil {
			return err
		}
		if _, err := io.WriteString(out, "\nyc = "); err != nil {
			return err
		}
		if err := t.displayFPATraceExpression(out, right); err != nil {
			return err
		}
		return diagnostic.WriteTraceEndl(out)
	})
}

func (t *SMTFloatingPointTheory) traceFPAAssignment(variable int32, isTrue bool) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	if variable < 0 || int(variable) >= len(state.boolExpr) {
		return ast.ErrASTDomain
	}
	return t.emitFPATrace("t_fpa", "assign_eh", 634, nil, func(out io.Writer) error {
		if _, err = fmt.Fprintf(out, "assign_eh for: %d (%t):\n", variable, isTrue); err != nil {
			return err
		}
		if err = t.displayFPATraceExpression(out, state.boolExpr[variable]); err != nil {
			return err
		}
		return diagnostic.WriteTraceEndl(out)
	})
}

func (t *SMTFloatingPointTheory) createFPAVariable(node *SMTEnode) (SMTTheoryVar, error) {
	variable, err := t.CreateVariable(node)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	err = t.emitFPATrace("t_fpa", "attach_new_th_var", 422, nil, func(out io.Writer) error {
		if _, writeErr := io.WriteString(out, "new theory var: "); writeErr != nil {
			return writeErr
		}
		if displayErr := t.displayFPATraceExpression(out, node.Owner()); displayErr != nil {
			return displayErr
		}
		_, writeErr := fmt.Fprintf(out, " := %d\n", variable)
		return writeErr
	})
	if err != nil {
		return SMTNullTheoryVar, err
	}
	return variable, nil
}

func (t *SMTFloatingPointTheory) traceFPAPop(count uint32, scopes int) error {
	return t.emitFPATrace("t_fpa", "pop_scope_eh", 625, nil, func(out io.Writer) error {
		_, err := fmt.Fprintf(out, "pop %d; now %d\n", count, scopes)
		return err
	})
}

func (t *SMTFloatingPointTheory) traceFPAInitializeModel(generator *SMTModelGenerator) error {
	return t.emitFPAModelTrace(generator, "t_fpa", "init_model", 723, nil, func(out io.Writer) error {
		if _, err := io.WriteString(out, "initializing model\n"); err != nil {
			return err
		}
		return t.Display(out)
	})
}

func (t *SMTFloatingPointTheory) traceFPAModelValue(generator *SMTModelGenerator, owner, sort ast.ASTHandle) error {
	return t.emitFPAModelTrace(generator, "t_fpa", "mk_value", 730, nil, func(out io.Writer) error {
		if _, err := io.WriteString(out, "mk_value for: "); err != nil {
			return err
		}
		if err := t.displayFPATraceExpression(out, owner); err != nil {
			return err
		}
		if _, err := io.WriteString(out, " (sort "); err != nil {
			return err
		}
		if err := t.displayFPATraceSort(out, sort); err != nil {
			return err
		}
		_, err := io.WriteString(out, ")\n")
		return err
	})
}

func (t *SMTFloatingPointTheory) traceFPAMissingDependency(generator *SMTModelGenerator, wrapped ast.ASTHandle, ownerInternalized bool) error {
	return t.emitFPAModelTrace(generator, "t_fpa_detail", "mk_value", 752, func() bool { return true }, func(out io.Writer) error {
		if _, err := io.WriteString(out, "Model dependency not internalized: "); err != nil {
			return err
		}
		if err := t.displayFPATraceExpression(out, wrapped); err != nil {
			return err
		}
		state := "not"
		if ownerInternalized {
			state = "is"
		}
		_, err := fmt.Fprintf(out, " (owner %s internalized)\n", state)
		return err
	})
}

func (t *SMTFloatingPointTheory) traceFPADependencies(generator *SMTModelGenerator, line int, dependencies ...*SMTEnode) error {
	return t.emitFPAModelTrace(generator, "t_fpa_detail", "mk_value", line, nil, func(out io.Writer) error {
		if _, err := io.WriteString(out, "Depends on: "); err != nil {
			return err
		}
		for i, dependency := range dependencies {
			if dependency == nil {
				return ast.ErrASTDomain
			}
			if i != 0 {
				if _, err := io.WriteString(out, "\n"); err != nil {
					return err
				}
			}
			if err := t.displayFPATraceExpression(out, dependency.Owner()); err != nil {
				return err
			}
			id, err := dependency.Root().OwnerID()
			if err != nil {
				return err
			}
			if _, err = fmt.Fprintf(out, " eq. cls. #%d", id); err != nil {
				return err
			}
		}
		return diagnostic.WriteTraceEndl(out)
	})
}

func (t *SMTFloatingPointTheory) traceFPAFinalCheck() error {
	return t.emitFPATrace("t_fpa", "final_check_eh", 717, nil, func(out io.Writer) error {
		_, err := io.WriteString(out, "final_check_eh\n")
		return err
	})
}
