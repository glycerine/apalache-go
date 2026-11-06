package smtcore

// Source: src/smt/theory_dense_diff_logic_def.h:663-706.

import (
	"fmt"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func smtDenseInfText(value *smtInfRational) string {
	main := value.rational.RatString()
	if value.infinitesimal.Sign() == 0 {
		return main
	}
	epsilon := value.infinitesimal.RatString()
	if value.infinitesimal.Sign() < 0 {
		var absolute big.Rat
		absolute.Neg(&value.infinitesimal)
		return "(" + main + " -e*" + absolute.RatString() + ")"
	}
	return "(" + main + " +e*" + epsilon + ")"
}

// Display retains theory_dense_diff_logic::display and display_atom.
func (t *SMTDenseDiffTheory) Display(out io.Writer) error {
	if _, err := t.State(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if err := smtWriteTheoryText(out, "Theory dense difference logic:\n"); err != nil {
		return err
	}
	if err := t.DisplayVariableEnodes(out); err != nil {
		return err
	}
	for source, row := range t.matrix.rows {
		for target, cell := range row {
			if cell.edge <= 0 {
				continue
			}
			sourceID, err := t.variables[source].OwnerID()
			if err != nil {
				return err
			}
			targetID, err := t.variables[target].OwnerID()
			if err != nil {
				return err
			}
			if err = smtWriteTheoryText(out, fmt.Sprintf("#%-5d -- %-10s : id%-5d --> #%d\n", sourceID, smtDenseInfText(&cell.distance), cell.edge, targetID)); err != nil {
				return err
			}
		}
	}
	if err := smtWriteTheoryText(out, "atoms:\n"); err != nil {
		return err
	}
	for index := range t.atoms {
		atom := &t.atoms[index]
		targetID, err := t.variables[atom.target].OwnerID()
		if err != nil {
			return err
		}
		sourceID, err := t.variables[atom.source].OwnerID()
		if err != nil {
			return err
		}
		assignment, err := t.state.Assignment(NewSMTLiteral(atom.variable, false))
		if err != nil {
			return err
		}
		if err = smtWriteTheoryText(out, fmt.Sprintf("#%-5d - #%-5d <= %-10s        assignment: %s\n", targetID, sourceID, smtDenseInfText(&atom.offset), assignment)); err != nil {
			return err
		}
	}
	return nil
}
