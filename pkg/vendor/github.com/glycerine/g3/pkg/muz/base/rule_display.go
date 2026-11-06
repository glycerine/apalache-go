package base

// Source: src/muz/base/dl_rule.cpp rule::display and src/muz/base/dl_util.cpp
// output_predicate.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// WriteRuleDisplay writes the human-readable Datalog rule form used by source
// diagnostic traces. The writer is borrowed.
func (m *RuleManager) WriteRuleDisplay(out io.Writer, rule *Rule) error {
	return m.WriteRuleDisplayWithProfile(out, rule, false)
}

// WriteRuleDisplayWithProfile writes a rule and its source accounting suffix
// when outputProfile is enabled. Fact rules keep the source's early return.
func (m *RuleManager) WriteRuleDisplayWithProfile(out io.Writer, rule *Rule, outputProfile bool) error {
	if out == nil {
		return io.ErrClosedPipe
	}
	if err := m.open(); err != nil {
		return err
	}
	state, err := rule.open()
	if err != nil {
		return err
	}
	if state.manager != m.manager {
		return ErrRuleOwner
	}
	name, err := rule.NameText()
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(out, "%s:\n", name); err != nil {
		return err
	}
	if err = m.writePredicateDisplay(out, state.head); err != nil {
		return err
	}
	if len(state.tail) == 0 {
		_, err = io.WriteString(out, ".\n")
		return err
	}
	if _, err = io.WriteString(out, " :- "); err != nil {
		return err
	}
	for index, tail := range state.tail {
		if index != 0 {
			if _, err = io.WriteString(out, ","); err != nil {
				return err
			}
		}
		if _, err = io.WriteString(out, "\n "); err != nil {
			return err
		}
		if tail.negated {
			if _, err = io.WriteString(out, "not "); err != nil {
				return err
			}
		}
		kind, x := m.manager.GetKind(tail.handle)
		if x != nil {
			return x
		}
		isPredicate := false
		if kind == ast.ASTApp {
			isPredicate, x = m.isPredicate(tail.handle)
			if x != nil {
				return x
			}
		}
		if isPredicate {
			err = m.writePredicateDisplay(out, tail.handle)
		} else {
			var printer *ast.SimpleASTPrinterContext
			printer, err = ast.NewSimpleASTPrinterContext(m.manager, out, out)
			if err == nil {
				err = printer.DisplayExpr(out, tail.handle, 0)
			}
		}
		if err != nil {
			return err
		}
	}
	if _, err = io.WriteString(out, "."); err != nil {
		return err
	}
	if outputProfile {
		if _, err = io.WriteString(out, " {"); err != nil {
			return err
		}
		if err = state.accounting.OutputProfile(out); err != nil {
			return err
		}
		if _, err = io.WriteString(out, "}"); err != nil {
			return err
		}
	}
	if _, err = io.WriteString(out, "\n"); err != nil {
		return err
	}
	proof, hasProof, err := rule.Proof()
	if err != nil || !hasProof {
		return err
	}
	printer, err := ast.NewSimpleASTPrinterContext(m.manager, out, out)
	if err != nil {
		return err
	}
	if err = printer.DisplayExpr(out, proof, 0); err != nil {
		return err
	}
	_, err = io.WriteString(out, "\n")
	return err
}

func (m *RuleManager) writePredicateDisplay(out io.Writer, predicate ast.ASTHandle) error {
	declaration, err := m.manager.GetDecl(predicate)
	if err != nil {
		return err
	}
	name, err := m.manager.GetASTName(declaration)
	if err != nil {
		return err
	}
	text, err := m.manager.Symbols.String(name)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(out, "%s(", text); err != nil {
		return err
	}
	count, err := m.manager.GetNumArgs(predicate)
	if err != nil {
		return err
	}
	for index := uint32(0); index < count; index++ {
		if index != 0 {
			if _, err = io.WriteString(out, ","); err != nil {
				return err
			}
		}
		argument, x := m.manager.GetArg(predicate, index)
		if x != nil {
			return x
		}
		kind, x := m.manager.GetKind(argument)
		if x != nil {
			return x
		}
		if kind == ast.ASTVar {
			variable, y := m.manager.GetVarIndex(argument)
			if y != nil {
				return y
			}
			_, err = fmt.Fprintf(out, "#%d", variable)
		} else {
			var printer *ast.SimpleASTPrinterContext
			printer, err = ast.NewSimpleASTPrinterContext(m.manager, out, out)
			if err == nil {
				err = printer.DisplayExpr(out, argument, 0)
			}
		}
		if err != nil {
			return err
		}
	}
	_, err = io.WriteString(out, ")")
	return err
}
