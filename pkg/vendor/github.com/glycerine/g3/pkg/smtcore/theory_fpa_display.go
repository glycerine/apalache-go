package smtcore

// Source: src/smt/theory_fpa.cpp:840-892 theory_fpa::display.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// Source endl flushes a borrowed buffered stream after its newline. The
// standalone display surface reports short writes and flush refusals.
func smtTheoryDisplayEndl(out io.Writer) error {
	if err := smtWriteTheoryText(out, "\n"); err != nil {
		return err
	}
	if stream, ok := out.(interface{ Flush() error }); ok {
		return stream.Flush()
	}
	return nil
}

func (t *SMTFloatingPointTheory) displayFamilyVariables(out io.Writer, family ast.FamilyID, heading string, conditional bool) (bool, error) {
	s, err := t.State()
	if err != nil {
		return false, err
	}
	first := true
	if !conditional {
		if err = smtWriteTheoryText(out, heading); err != nil {
			return false, err
		}
		if err = smtTheoryDisplayEndl(out); err != nil {
			return false, err
		}
	}
	for _, node := range s.enodeStack {
		variable := node.TheoryVar(family)
		if variable == SMTNullTheoryVar {
			continue
		}
		if first && conditional {
			if err = smtWriteTheoryText(out, heading); err != nil {
				return false, err
			}
			if err = smtTheoryDisplayEndl(out); err != nil {
				return false, err
			}
		}
		if err = smtWriteTheoryText(out, fmt.Sprintf("%d -> ", variable)); err != nil {
			return false, err
		}
		printer, err := ast.NewSimpleASTPrinterContext(s.manager, out, out)
		if err != nil {
			return false, err
		}
		if err = printer.DisplayExpr(smtContextDisplayWriter{out}, node.owner, 0); err != nil {
			return false, err
		}
		if err = smtTheoryDisplayEndl(out); err != nil {
			return false, err
		}
		first = false
	}
	return !first, nil
}

func (t *SMTFloatingPointTheory) Display(out io.Writer) error {
	if t == nil || t.SMTTheoryBase == nil || out == nil {
		return ast.ErrASTHandle
	}
	s, err := t.State()
	if err != nil {
		return err
	}
	found, err := t.displayFamilyVariables(out, t.FamilyID(), "fpa theory variables:", true)
	if err != nil || !found {
		return err
	}
	// Converter construction already registers both families. These lookups
	// borrow their existing identities and do not install plugins during display.
	for _, section := range []struct{ name, heading string }{{"bv", "bv theory variables:"}, {"arith", "arith theory variables:"}} {
		name, err := s.manager.Symbols.Intern(section.name)
		if err != nil {
			return err
		}
		family, err := s.manager.FamilyIDForSymbol(name)
		if err != nil {
			return err
		}
		if _, err = t.displayFamilyVariables(out, family, section.heading, false); err != nil {
			return err
		}
	}
	if err = smtWriteTheoryText(out, "equivalence classes:\n"); err != nil {
		return err
	}
	for _, node := range s.enodeStack {
		id, err := node.Root().OwnerID()
		if err != nil {
			return err
		}
		if err = smtWriteTheoryText(out, fmt.Sprintf("%d --> ", id)); err != nil {
			return err
		}
		printer, err := ast.NewSimpleASTPrinterContext(s.manager, out, out)
		if err != nil {
			return err
		}
		if err = printer.DisplayExpr(smtContextDisplayWriter{out}, node.owner, 0); err != nil {
			return err
		}
		if err = smtTheoryDisplayEndl(out); err != nil {
			return err
		}
	}
	return nil
}
