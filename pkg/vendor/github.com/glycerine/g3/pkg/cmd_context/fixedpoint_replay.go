package cmd_context

// Replayable SMT-LIB export for the retained fixedpoint context.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// WriteFixedpointReplaySMT2 writes the effective current fixedpoint declarations,
// background assertions, and Horn rules. It deliberately omits a query.
func (c *Context) WriteFixedpointReplaySMT2(out io.Writer) (resultErr error) {
	if out == nil {
		return ast.ErrASTHandle
	}
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	predicates, err := engine.Predicates()
	if err != nil {
		return err
	}
	assertions, err := engine.BackgroundAssertions()
	if err != nil {
		return err
	}
	defer func() {
		for i := len(assertions) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&assertions[i]))
		}
	}()
	rules, names, err := engine.NamedRulesWithSymbols()
	if err != nil {
		return err
	}
	defer func() {
		for i := len(rules) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&rules[i]))
		}
	}()
	facts, err := engine.StoredFactFormulas()
	if err != nil {
		return err
	}
	defer func() {
		for i := len(facts) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&facts[i]))
		}
	}()
	util, err := ast.NewASTPPUtil(c.manager)
	if err != nil {
		return err
	}
	for _, predicate := range predicates {
		if err = util.Collect(predicate); err != nil {
			return err
		}
		if err = util.RemoveDeclaration(predicate); err != nil {
			return err
		}
	}
	for _, reference := range assertions {
		handle, x := c.manager.RefGet(reference)
		if x != nil {
			return x
		}
		if err = util.Collect(handle); err != nil {
			return err
		}
	}
	for _, reference := range rules {
		handle, x := c.manager.RefGet(reference)
		if x != nil {
			return x
		}
		if err = util.Collect(handle); err != nil {
			return err
		}
	}
	for _, reference := range facts {
		handle, x := c.manager.RefGet(reference)
		if x != nil {
			return x
		}
		if err = util.Collect(handle); err != nil {
			return err
		}
	}
	if err = c.replayRemoveSortConstructorInstances(util); err != nil {
		return err
	}
	if err = util.DisplayDeclarations(out); err != nil {
		return err
	}
	printer, err := ast.NewLegacySMTPrinter(c.manager)
	if err != nil {
		return err
	}
	for _, predicate := range predicates {
		symbol, x := c.manager.GetASTName(predicate)
		if x != nil {
			return x
		}
		name, x := c.smt2SymbolName(symbol)
		if x != nil {
			return x
		}
		arity, x := c.manager.GetArity(predicate)
		if x != nil {
			return x
		}
		if _, x = fmt.Fprintf(out, "(declare-rel %s (", name); x != nil {
			return x
		}
		for i := uint32(0); i < arity; i++ {
			if i != 0 {
				if _, x = io.WriteString(out, " "); x != nil {
					return x
				}
			}
			sort, y := c.manager.GetDomain(predicate, i)
			if y != nil {
				return y
			}
			if y = printer.DisplaySort(out, sort); y != nil {
				return y
			}
		}
		if _, x = io.WriteString(out, "))\n"); x != nil {
			return x
		}
	}
	for _, reference := range assertions {
		handle, x := c.manager.RefGet(reference)
		if x != nil {
			return x
		}
		if _, x = io.WriteString(out, "(assert "); x != nil {
			return x
		}
		if x = c.WriteExpression(out, handle); x != nil {
			return x
		}
		if _, x = io.WriteString(out, ")\n"); x != nil {
			return x
		}
	}
	for _, reference := range facts {
		handle, x := c.manager.RefGet(reference)
		if x != nil {
			return x
		}
		if _, x = io.WriteString(out, "(rule "); x != nil {
			return x
		}
		if x = c.WriteExpression(out, handle); x != nil {
			return x
		}
		if _, x = io.WriteString(out, ")\n"); x != nil {
			return x
		}
	}
	for i, reference := range rules {
		handle, x := c.manager.RefGet(reference)
		if x != nil {
			return x
		}
		if _, x = io.WriteString(out, "(rule "); x != nil {
			return x
		}
		if x = c.WriteExpression(out, handle); x != nil {
			return x
		}
		// Rule names are command arguments. A :named expression annotation is
		// display syntax and does not restore the rule's label on replay.
		if !names[i].IsNull() {
			name, y := c.smt2SymbolName(names[i])
			if y != nil {
				return y
			}
			// The legacy display quoting helper leaves empty text bare;
			// replay needs an explicit token for a present empty symbol.
			if name == "" {
				name = "||"
			}
			if _, y = fmt.Fprintf(out, " %s", name); y != nil {
				return y
			}
		}
		if _, x = io.WriteString(out, ")\n"); x != nil {
			return x
		}
	}
	return nil
}
