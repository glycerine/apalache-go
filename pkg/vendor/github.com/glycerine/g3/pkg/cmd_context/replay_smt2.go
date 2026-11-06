package cmd_context

// Replayable SMT-LIB state export for the retained command context.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

type replayBoundary struct {
	functions, sorts, assertions, sortConstructors int
}

func (c *Context) replayBoundaries() []replayBoundary {
	result := make([]replayBoundary, 0, len(c.scopes)+1)
	for _, value := range c.scopes {
		result = append(result, replayBoundary{value.functions, value.sorts, value.assertions, value.sortConstructors})
	}
	return append(result, replayBoundary{len(c.functionTrail), len(c.sortTrail), len(c.assertions), len(c.sortConstructorTrail)})
}

func replaySliceBounds(level int, ends []replayBoundary) (replayBoundary, replayBoundary) {
	if level == 0 {
		return replayBoundary{}, ends[0]
	}
	return ends[level-1], ends[level]
}

func (c *Context) replayMarkCollected(util *ast.ASTPPUtil, declarations, sorts map[ast.ASTHandle]bool) error {
	values, err := util.CollectedDeclarations()
	if err != nil {
		return err
	}
	for _, value := range values {
		declarations[value] = true
	}
	values, err = util.CollectedSorts()
	if err != nil {
		return err
	}
	for _, value := range values {
		sorts[value] = true
	}
	return nil
}

func (c *Context) replayRemoveCollected(util *ast.ASTPPUtil, declarations, sorts map[ast.ASTHandle]bool) error {
	for value := range declarations {
		if err := util.RemoveDeclaration(value); err != nil {
			return err
		}
	}
	for value := range sorts {
		if err := util.RemoveSort(value); err != nil {
			return err
		}
	}
	return nil
}

func (c *Context) replayRemoveSortConstructorInstances(util *ast.ASTPPUtil) error {
	sorts, err := util.CollectedSorts()
	if err != nil {
		return err
	}
	for _, sort := range sorts {
		name, x := c.manager.GetASTName(sort)
		if x != nil {
			return x
		}
		spelling, x := c.manager.Symbols.String(name)
		if x != nil {
			return x
		}
		if _, found := c.sortConstructors[spelling]; found {
			if x = util.RemoveSort(sort); x != nil {
				return x
			}
		}
	}
	return nil
}

func (c *Context) writeReplaySortDeclarations(out io.Writer, begin, end replayBoundary, emittedSorts map[ast.ASTHandle]bool) error {
	util, err := ast.NewASTPPUtil(c.manager)
	if err != nil {
		return err
	}
	aliases := make([]declaration, 0)
	for _, row := range c.sortTrail[begin.sorts:end.sorts] {
		name, e := c.manager.GetASTName(row.handle)
		if e != nil {
			return e
		}
		spelling, e := c.manager.Symbols.String(name)
		if e != nil {
			return e
		}
		if spelling != row.name {
			aliases = append(aliases, row)
		}
		if e = util.Collect(row.handle); e != nil {
			return e
		}
	}
	for _, name := range c.sortConstructorTrail[begin.sortConstructors:end.sortConstructors] {
		constructor := c.sortConstructors[name]
		if constructor.definition {
			if err = util.Collect(constructor.body); err != nil {
				return err
			}
			for _, formal := range constructor.formals {
				if err = util.RemoveSort(formal); err != nil {
					return err
				}
			}
		}
	}
	if err = c.replayRemoveCollected(util, nil, emittedSorts); err != nil {
		return err
	}
	if err = c.replayRemoveSortConstructorInstances(util); err != nil {
		return err
	}
	if err = util.DisplayDeclarations(out); err != nil {
		return err
	}
	if err = c.replayMarkCollected(util, make(map[ast.ASTHandle]bool), emittedSorts); err != nil {
		return err
	}
	printer, err := ast.NewLegacySMTPrinter(c.manager)
	if err != nil {
		return err
	}
	for _, row := range aliases {
		name, e := c.manager.Symbols.Intern(row.name)
		if e != nil {
			return e
		}
		spelling, e := c.smt2SymbolName(name)
		if e != nil {
			return e
		}
		if _, e = fmt.Fprintf(out, "(define-sort %s () ", spelling); e != nil {
			return e
		}
		if e = printer.DisplaySort(out, row.handle); e != nil {
			return e
		}
		if _, e = io.WriteString(out, ")\n"); e != nil {
			return e
		}
	}
	for _, name := range c.sortConstructorTrail[begin.sortConstructors:end.sortConstructors] {
		constructor := c.sortConstructors[name]
		spelling, e := c.smt2SymbolName(constructor.symbol)
		if e != nil {
			return e
		}
		if !constructor.definition {
			if _, e = fmt.Fprintf(out, "(declare-sort %s %d)\n", spelling, constructor.arity); e != nil {
				return e
			}
			continue
		}
		if _, e = fmt.Fprintf(out, "(define-sort %s (", spelling); e != nil {
			return e
		}
		for index, formal := range constructor.formals {
			if index != 0 {
				if _, e = io.WriteString(out, " "); e != nil {
					return e
				}
			}
			formalName, x := c.manager.GetASTName(formal)
			if x != nil {
				return x
			}
			formalSpelling, x := c.smt2SymbolName(formalName)
			if x != nil {
				return x
			}
			if _, x = io.WriteString(out, formalSpelling); x != nil {
				return x
			}
		}
		if _, e = io.WriteString(out, ") "); e != nil {
			return e
		}
		if e = printer.DisplaySort(out, constructor.body); e != nil {
			return e
		}
		if _, e = io.WriteString(out, ")\n"); e != nil {
			return e
		}
	}
	return nil
}

func (c *Context) writeReplayFormulaDeclarations(out io.Writer, begin, end replayBoundary, emittedDecls, emittedSorts map[ast.ASTHandle]bool) error {
	util, err := ast.NewASTPPUtil(c.manager)
	if err != nil {
		return err
	}
	for _, row := range c.functionTrail[begin.functions:end.functions] {
		if err = util.Collect(row.handle); err != nil {
			return err
		}
	}
	for _, row := range c.assertions[begin.assertions:end.assertions] {
		if err = util.Collect(row.expression); err != nil {
			return err
		}
	}
	if err = c.replayRemoveCollected(util, emittedDecls, emittedSorts); err != nil {
		return err
	}
	if err = c.replayRemoveSortConstructorInstances(util); err != nil {
		return err
	}
	if err = util.DisplayDeclarations(out); err != nil {
		return err
	}
	return c.replayMarkCollected(util, emittedDecls, emittedSorts)
}

func (c *Context) writeReplayAssertions(out io.Writer, begin, end replayBoundary) error {
	for _, row := range c.assertions[begin.assertions:end.assertions] {
		if _, err := io.WriteString(out, "(assert "); err != nil {
			return err
		}
		if !row.tag.IsNull() {
			if _, err := io.WriteString(out, "(! "); err != nil {
				return err
			}
		}
		if err := c.WriteExpression(out, row.expression); err != nil {
			return err
		}
		if !row.tag.IsNull() {
			if _, err := io.WriteString(out, " :named "); err != nil {
				return err
			}
			if err := c.WriteExpression(out, row.tag); err != nil {
				return err
			}
			if _, err := io.WriteString(out, ")"); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(out, ")\n"); err != nil {
			return err
		}
	}
	return nil
}

// WriteReplaySMT2 writes a command stream that reconstructs the current live
// declarations, assertions, named assertions, and active scope stack. It does
// not append check-sat, so replay leaves the destination in the same mutable
// state for subsequent commands.
func (c *Context) WriteReplaySMT2(out io.Writer) error {
	if err := c.valid(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if _, err := c.Manager(); err != nil {
		return err
	}
	state, err := c.parameters.State()
	if err != nil {
		return err
	}
	for _, option := range []struct {
		name  string
		value bool
	}{
		{"produce-models", state.Model},
		{"produce-proofs", state.Proof},
		{"produce-unsat-cores", state.UnsatCore},
		{"global-declarations", c.GlobalDeclarations},
	} {
		if _, err = fmt.Fprintf(out, "(set-option :%s %t)\n", option.name, option.value); err != nil {
			return err
		}
	}
	if c.logic != "" {
		if _, err = fmt.Fprintf(out, "(set-logic %s)\n", c.logic); err != nil {
			return err
		}
	}
	ends := c.replayBoundaries()
	emittedDecls := make(map[ast.ASTHandle]bool)
	emittedSorts := make(map[ast.ASTHandle]bool)
	for level := range ends {
		if level != 0 {
			if _, err := io.WriteString(out, "(push 1)\n"); err != nil {
				return err
			}
		}
		begin, end := replaySliceBounds(level, ends)
		if err := c.writeReplaySortDeclarations(out, begin, end, emittedSorts); err != nil {
			return err
		}
		if err := c.writeReplayFormulaDeclarations(out, begin, end, emittedDecls, emittedSorts); err != nil {
			return err
		}
		if err := c.writeReplayAssertions(out, begin, end); err != nil {
			return err
		}
	}
	return nil
}
