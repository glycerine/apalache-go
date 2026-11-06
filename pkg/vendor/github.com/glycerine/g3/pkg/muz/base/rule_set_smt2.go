package base

// Source: src/muz/base/dl_context.cpp context::display_smt2 plain HORN
// assertion formatting. This explicit RuleSet export includes every rule;
// it is not the context dump with output-query conversion or default options.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// WriteSMT2 writes a RuleSet as the source fixedpoint SMT-LIB HORN dump.
// It borrows the rule set and writer.
func (s *RuleSet) WriteSMT2(out io.Writer) (resultErr error) {
	if out == nil {
		return io.ErrClosedPipe
	}
	if err := s.open(); err != nil {
		return err
	}
	manager := s.manager.manager
	util, err := ast.NewASTPPUtil(manager)
	if err != nil {
		return err
	}
	printer, err := ast.NewLegacySMTPrinter(manager)
	if err != nil {
		return err
	}
	type sourceRule struct {
		formula ast.ASTRef
		name    symbol.Symbol
	}
	rules := make([]sourceRule, 0, len(s.rules))
	defer func() {
		for i := len(rules) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, manager.DelRef(&rules[i].formula))
		}
	}()
	for _, rule := range s.rules {
		formula, e := s.manager.RuleFormula(rule)
		if e != nil {
			return e
		}
		handle, e := manager.RefGet(formula)
		if e == nil {
			e = util.Collect(handle)
		}
		if e != nil {
			return errors.Join(e, manager.DelRef(&formula))
		}
		name, e := rule.NameSymbol()
		if e != nil {
			return errors.Join(e, manager.DelRef(&formula))
		}
		rules = append(rules, sourceRule{formula: formula, name: name})
	}
	if _, err = io.WriteString(out, "(set-logic HORN)\n"); err != nil {
		return err
	}
	if err = util.DisplayDeclarations(out); err != nil {
		return err
	}
	declarations, err := util.CollectedDeclarations()
	if err != nil {
		return err
	}
	usedNames := make(map[symbol.Symbol]struct{}, len(declarations)+len(rules))
	for _, declaration := range declarations {
		symbol, e := manager.GetASTName(declaration)
		if e != nil {
			return e
		}
		usedNames[symbol] = struct{}{}
	}
	for _, item := range rules {
		if _, err = io.WriteString(out, "(assert "); err != nil {
			return err
		}
		handle, e := manager.RefGet(item.formula)
		if e != nil {
			return e
		}
		if item.name != (symbol.Symbol{}) {
			name := item.name
			if _, err = io.WriteString(out, "(! "); err != nil {
				return err
			}
			if err = printer.DisplayExpr(out, handle, nil); err != nil {
				return err
			}
			if _, err = io.WriteString(out, " :named "); err != nil {
				return err
			}
			for {
				if _, found := usedNames[name]; !found {
					break
				}
				text, e := manager.Symbols.Display(name)
				if e != nil {
					return e
				}
				name, e = manager.Symbols.Intern(text + "!")
				if e != nil {
					return e
				}
			}
			usedNames[name] = struct{}{}
			if err = s.writeContextName(out, name); err != nil {
				return err
			}
			if _, err = io.WriteString(out, ")"); err != nil {
				return err
			}
		} else if err = printer.DisplayExpr(out, handle, nil); err != nil {
			return err
		}
		if _, err = io.WriteString(out, ")\n"); err != nil {
			return err
		}
	}
	return nil
}
