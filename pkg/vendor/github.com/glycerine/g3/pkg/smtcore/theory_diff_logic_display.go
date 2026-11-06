package smtcore

// Source: src/smt/theory_diff_logic_def.h:38-52,920-925 and
// src/smt/diff_logic.h:846-922.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (g *SMTRationalDiffGraph) display(out io.Writer) error {
	if g == nil || out == nil {
		return ast.ErrASTHandle
	}
	for i := range g.edges {
		edge := &g.edges[i]
		if !edge.enabled {
			continue
		}
		if err := smtWriteTheoryText(out, fmt.Sprintf("%s (<= (- $%d $%d) %s) %d\n", smtUTVPILiteralText(edge.explanation), edge.target, edge.source, edge.weight.RatString(), edge.timestamp)); err != nil {
			return err
		}
	}
	for variable := range g.assignment {
		if err := smtWriteTheoryText(out, fmt.Sprintf("$%d := %s\n", variable, g.assignment[variable].RatString())); err != nil {
			return err
		}
	}
	return nil
}

func (g *SMTInfRationalDiffGraph) display(out io.Writer) error {
	if g == nil || out == nil {
		return ast.ErrASTHandle
	}
	for i := range g.edges {
		edge := &g.edges[i]
		if !edge.enabled {
			continue
		}
		if err := smtWriteTheoryText(out, fmt.Sprintf("%s (<= (- $%d $%d) %s) %d\n", smtUTVPILiteralText(edge.explanation), edge.target, edge.source, smtDenseInfText(&edge.weight), edge.timestamp)); err != nil {
			return err
		}
	}
	for variable := range g.assignment {
		if err := smtWriteTheoryText(out, fmt.Sprintf("$%d := %s\n", variable, smtDenseInfText(&g.assignment[variable]))); err != nil {
			return err
		}
	}
	return nil
}

func displaySMTDiffAtom(out io.Writer, state *SMTInternalizerState, printer *ast.SimpleASTPrinterContext, atom *smtIntegerDiffAtom) error {
	assignment, err := state.Assignment(NewSMTLiteral(atom.variable, false))
	if err != nil {
		return err
	}
	sign := assignment == lbool.Undef || atom.truth
	if err = smtWriteTheoryText(out, smtUTVPILiteralText(NewSMTLiteral(atom.variable, sign))+" "); err != nil {
		return err
	}
	expression, err := state.BoolExpression(atom.variable)
	if err != nil {
		return err
	}
	if err = printer.DisplayExpr(out, expression, 0); err != nil {
		return err
	}
	return smtWriteTheoryText(out, " ")
}

// Display retains theory_diff_logic::display for the concrete integer graph.
func (t *SMTIntegerDiffTheory) Display(out io.Writer) error {
	if t == nil || t.SMTTheoryBase == nil || t.graph == nil || out == nil {
		return ast.ErrASTHandle
	}
	state, err := t.State()
	if err != nil {
		return err
	}
	out = smtContextDisplayWriter{out}
	printer, err := ast.NewSimpleASTPrinterContext(state.manager, out, out)
	if err != nil {
		return err
	}
	for i := range t.atoms {
		if err = displaySMTDiffAtom(out, state, printer, &t.atoms[i]); err != nil {
			return err
		}
	}
	return t.graph.display(out)
}

// Display retains theory_diff_logic::display for the concrete real/mixed graph.
func (t *SMTRealDiffTheory) Display(out io.Writer) error {
	if t == nil || t.SMTIntegerDiffTheory == nil || t.realGraph == nil || out == nil {
		return ast.ErrASTHandle
	}
	state, err := t.State()
	if err != nil {
		return err
	}
	out = smtContextDisplayWriter{out}
	printer, err := ast.NewSimpleASTPrinterContext(state.manager, out, out)
	if err != nil {
		return err
	}
	for i := range t.realAtoms {
		if err = displaySMTDiffAtom(out, state, printer, &t.realAtoms[i]); err != nil {
			return err
		}
	}
	return t.realGraph.display(out)
}
