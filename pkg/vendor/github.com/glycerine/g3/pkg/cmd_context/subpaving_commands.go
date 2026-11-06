package cmd_context

// Source: src/cmd_context/extra_cmds/subpaving_cmds.cpp, to_subpaving.

import (
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	subpaving "github.com/glycerine/g3/pkg/math/subpaving/tactic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// WriteSubpaving simplifies and internalizes a term into a temporary rational
// subpaving context, then prints its mappings and generated constraints.
func (c *Context) WriteSubpaving(out io.Writer, expression ast.ASTHandle) (resultErr error) {
	if out == nil {
		return ast.ErrASTHandle
	}
	m, err := c.Manager()
	if err != nil {
		return err
	}
	globals, err := c.GlobalParameters()
	if err != nil {
		return err
	}
	pc := globals.Context
	params, err := pc.NewValue()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, pc.Del(&params)) }()
	q, err := m.NumeralService().NewPrivateRationalManager()
	if err != nil {
		if q != nil {
			err = errors.Join(err, q.CloseChecked())
		}
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, q.CloseChecked()) }()
	engine, err := subpaving.NewRationalEngine(m.ResourceLimit(), q, pc, &params)
	if err != nil {
		if engine != nil {
			err = errors.Join(err, engine.Close())
		}
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, engine.Close()) }()
	mapping, err := ast.NewExpr2Var(m)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, mapping.Close()) }()
	converter, err := subpaving.NewRationalExprConverterWithMapping(m, engine, q, mapping)
	if err != nil {
		if converter != nil {
			err = errors.Join(err, converter.Close())
		}
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, converter.Close()) }()
	name := "mul_to_power"
	if err = pc.SetBoolName(&params, &name, true); err != nil {
		return err
	}
	simplifier, err := rewriter.NewTheoryRewriter(m, pc, globals, &params)
	if err != nil {
		if simplifier != nil {
			err = errors.Join(err, simplifier.Close())
		}
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, simplifier.Close()) }()
	ref, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&ref)) }()
	if err = simplifier.Rewrite(expression, ref); err != nil {
		return err
	}
	simplified, err := m.RefGet(ref)
	if err != nil {
		return err
	}
	if err = c.WriteExpression(out, simplified); err != nil {
		return err
	}
	if _, err = fmt.Fprint(out, "\n=======>\n"); err != nil {
		return err
	}
	var numerator, denominator numeral.Int
	defer func() {
		resultErr = errors.Join(resultErr, q.Integers().Del(&denominator), q.Integers().Del(&numerator))
	}()
	variable, err := converter.InternalizeTerm(simplified, &numerator, &denominator)
	if err != nil {
		return err
	}
	entries, err := mapping.Entries()
	if err != nil {
		return err
	}
	// The C++ hash table has no ordering contract. Use stable variable order
	// for the Go map snapshot so repeated runs have deterministic output.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Variable < entries[j].Variable })
	for _, entry := range entries {
		if _, err = fmt.Fprintf(out, "x%d := ", entry.Variable); err != nil {
			return err
		}
		if err = c.WriteExpression(out, entry.Expr); err != nil {
			return err
		}
		if _, err = fmt.Fprintln(out); err != nil {
			return err
		}
	}
	context, err := engine.Context()
	if err != nil {
		return err
	}
	if err = context.DisplayConstraints(out, false); err != nil {
		return err
	}
	n, err := q.Integers().String(&numerator)
	if err != nil {
		return err
	}
	d, err := q.Integers().String(&denominator)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "%s/%s x%d\n", n, d, variable)
	return err
}
