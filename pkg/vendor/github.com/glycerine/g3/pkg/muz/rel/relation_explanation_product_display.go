package rel

// Source: src/muz/rel/dl_base.h relation_base::display_tuples,
// src/muz/rel/dl_product_relation.cpp product_relation::display, and
// src/muz/rel/dl_sieve_relation.cpp sieve_relation::display.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// Display writes the concrete two-sieve product used for relation-level
// explanations. The finite component precedes the whole-relation proof,
// matching assign_rel_level_kind's source product specification.
func (p *RelationExplanationProduct) Display(out io.Writer) error {
	if err := p.open(); err != nil {
		return err
	}
	if out == nil {
		return ErrTableRelation
	}
	if _, err := io.WriteString(out, "Product of the following relations:\n"); err != nil {
		return err
	}
	return p.displayComponents(out)
}

func (p *RelationExplanationProduct) displayComponents(out io.Writer) error {
	if _, err := io.WriteString(out, "Sieve relation ("); err != nil {
		return err
	}
	for column := range p.finiteSorts {
		if column != 0 {
			if _, err := io.WriteString(out, ","); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(out, column); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(out, ")\n"); err != nil {
		return err
	}
	if err := p.displayFiniteComponent(out); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Sieve relation (%d)\n", len(p.finiteSorts)); err != nil {
		return err
	}
	return p.explanation.Display(out)
}

func (p *RelationExplanationProduct) displayFiniteComponent(out io.Writer) error {
	// check_relation::display appends its formula after the concrete relation.
	// Keep that diagnostic for the two live formula-checked finite backings.
	switch table := p.table.(type) {
	case *FormulaCheckedTable:
		return table.Display(out)
	case *FormulaCheckedFinitePair:
		if err := table.open(); err != nil {
			return err
		}
		if err := displayRelationExplanationTable(table.pair, len(p.finiteSorts), out); err != nil {
			return err
		}
		return displayRelationExplanationFormula(table.formula, out)
	}
	return displayRelationExplanationTable(p.table, len(p.finiteSorts), out)
}

func displayRelationExplanationTable(table FiniteTable, width int, out io.Writer) error {
	signature, err := table.Signature()
	if err != nil {
		return err
	}
	if signature.Len() != width {
		return ErrTableRelation
	}
	if _, err = io.WriteString(out, "table with signature ("); err != nil {
		return err
	}
	for column, domain := range signature.domains {
		if column != 0 {
			if _, err = io.WriteString(out, ","); err != nil {
				return err
			}
		}
		if _, err = fmt.Fprint(out, domain); err != nil {
			return err
		}
	}
	if _, err = io.WriteString(out, "):\n"); err != nil {
		return err
	}
	facts, err := table.Facts()
	if err != nil {
		return err
	}
	for _, fact := range facts {
		if len(fact) != signature.Len() {
			return ErrTableRelation
		}
		if _, err = io.WriteString(out, "("); err != nil {
			return err
		}
		for column, value := range fact {
			if column != 0 {
				if _, err = io.WriteString(out, ","); err != nil {
					return err
				}
			}
			if _, err = fmt.Fprint(out, value); err != nil {
				return err
			}
		}
		if _, err = io.WriteString(out, ")\n"); err != nil {
			return err
		}
	}
	if _, err = io.WriteString(out, "\n"); err != nil {
		return err
	}
	return nil
}

func displayRelationExplanationFormula(relation *FormulaCheckedTable, out io.Writer) error {
	if err := relation.open(); err != nil {
		return err
	}
	formula, err := relation.manager.RefGet(relation.formula)
	if err != nil {
		return err
	}
	printer, err := ast.NewSimpleASTPrinterContext(relation.manager, out, out)
	if err != nil {
		return err
	}
	if err = printer.DisplayExpr(out, formula, 0); err != nil {
		return err
	}
	_, err = io.WriteString(out, "\n")
	return err
}

// DisplayTuples follows relation_base's default tuple display: it writes the
// predicate heading and then delegates to the structural product display.
func (p *RelationExplanationProduct) DisplayTuples(context *base.DatalogContext, predicate ast.ASTHandle, out io.Writer) error {
	if err := p.open(); err != nil {
		return err
	}
	if context == nil || out == nil {
		return ErrTableRelation
	}
	manager, err := context.Manager()
	if err != nil {
		return err
	}
	if manager != p.manager.manager {
		return ErrTableRelation
	}
	if _, err = p.manager.validateRelationExplanation(predicate, p); err != nil {
		return err
	}
	name, err := manager.GetASTName(predicate)
	if err != nil {
		return err
	}
	nameText, err := manager.Symbols.Display(name)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(out, "Tuples in %s: \n", nameText); err != nil {
		return err
	}
	return p.Display(out)
}
