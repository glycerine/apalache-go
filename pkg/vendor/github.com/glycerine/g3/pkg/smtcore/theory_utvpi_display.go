package smtcore

// Source: src/smt/theory_utvpi_def.h:82-94,461-466 and
// src/smt/diff_logic.h:888-922.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func smtUTVPILiteralText(literal SMTLiteral) string {
	switch literal {
	case SMTTrueLiteral:
		return "true"
	case SMTFalseLiteral:
		return "false"
	}
	if literal.Sign() {
		return fmt.Sprintf("(not p%d)", literal.Var())
	}
	return fmt.Sprintf("p%d", literal.Var())
}

func (g *smtUTVPIGraph) display(out io.Writer) error {
	if g == nil || g.graph == nil || out == nil {
		return ast.ErrASTHandle
	}
	for i := range g.graph.edges {
		edge := &g.graph.edges[i]
		if !edge.enabled {
			continue
		}
		if err := smtWriteTheoryText(out, fmt.Sprintf("%s (<= (- $%d $%d) %s) %d\n", smtUTVPILiteralText(edge.explanation), edge.target, edge.source, smtDenseInfText(&edge.weight), edge.timestamp)); err != nil {
			return err
		}
	}
	for variable := range g.graph.assignment {
		if err := smtWriteTheoryText(out, fmt.Sprintf("$%d := %s\n", variable, smtDenseInfText(&g.graph.assignment[variable]))); err != nil {
			return err
		}
	}
	return nil
}

// Display retains theory_utvpi::display. The source atom printer returns
// immediately after the positive literal and expression, before its dead
// assignment/edge branch.
func (t *SMTUTVPITheory) Display(out io.Writer) error {
	if t == nil || t.SMTTheoryBase == nil || out == nil {
		return ast.ErrASTHandle
	}
	s, err := t.State()
	if err != nil {
		return err
	}
	out = smtContextDisplayWriter{out}
	printer, err := ast.NewSimpleASTPrinterContext(s.manager, out, out)
	if err != nil {
		return err
	}
	for i := range t.atoms {
		atom := &t.atoms[i]
		if err = smtWriteTheoryText(out, fmt.Sprintf("p%d ", atom.variable)); err != nil {
			return err
		}
		expression, err := s.BoolExpression(atom.variable)
		if err != nil {
			return err
		}
		if err = printer.DisplayExpr(out, expression, 0); err != nil {
			return err
		}
		if err = smtWriteTheoryText(out, " \n"); err != nil {
			return err
		}
	}
	return t.graph.display(out)
}
