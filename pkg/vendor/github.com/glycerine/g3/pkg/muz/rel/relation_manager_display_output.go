package rel

// Source: src/muz/rel/dl_relation_manager.cpp
// relation_manager::display_output_tables.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// displayFiniteTableTuples is the concrete finite-table instance of
// relation_base::display_tuples. The table remains borrowed; Facts returns
// detached rows and forces a lazy table in the same way source iteration does.
func (m *RelationManager) displayFiniteTableTuples(context *base.DatalogContext, predicate ast.ASTHandle, table FiniteTable, out io.Writer) error {
	if table == nil || context == nil || out == nil {
		return ErrTableRelation
	}
	manager, err := context.Manager()
	if err != nil {
		return err
	}
	if manager != m.manager {
		return ErrTableRelation
	}
	sorts, expected, err := m.PredicateSignature(predicate)
	if err != nil {
		return err
	}
	actual, err := table.Signature()
	if err != nil {
		return err
	}
	if !expected.equal(actual) {
		return ErrTableRelation
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
	facts, err := table.Facts()
	if err != nil {
		return err
	}
	for _, fact := range facts {
		if len(fact) != len(sorts) {
			return ErrTableRelation
		}
		if _, err = io.WriteString(out, "\t("); err != nil {
			return err
		}
		for index, value := range fact {
			if index != 0 {
				if _, err = io.WriteString(out, ","); err != nil {
					return err
				}
			}
			argument, nameErr := context.ArgumentName(predicate, uint32(index))
			if nameErr != nil {
				return nameErr
			}
			argumentText, displayErr := manager.Symbols.Display(argument)
			if displayErr != nil {
				return displayErr
			}
			if _, err = fmt.Fprintf(out, "%s=", argumentText); err != nil {
				return err
			}
			if err = context.WriteConstant(out, sorts[index], uint64(value)); err != nil {
				return err
			}
			if _, err = fmt.Fprintf(out, "(%d)", value); err != nil {
				return err
			}
		}
		if _, err = io.WriteString(out, ")\n"); err != nil {
			return err
		}
	}
	return nil
}

// DisplayOutputTables writes each requested output relation in rule-set order.
// Missing relations still receive the source tuple heading.
func (m *RelationManager) DisplayOutputTables(context *base.DatalogContext, rules *base.RuleSet, out io.Writer) error {
	if err := m.open(); err != nil {
		return err
	}
	if context == nil || rules == nil || out == nil {
		return ErrTableRelation
	}
	manager, err := context.Manager()
	if err != nil {
		return err
	}
	if manager != m.manager {
		return ErrTableRelation
	}
	outputs, err := rules.OutputPredicates()
	if err != nil {
		return err
	}
	for _, predicate := range outputs {
		if product := m.explanations[predicate]; product != nil {
			if err = product.DisplayTuples(context, predicate, out); err != nil {
				return err
			}
			continue
		}
		if product := m.relationLevel[predicate]; product != nil {
			if err = product.DisplayTuples(context, predicate, out); err != nil {
				return err
			}
			continue
		}
		if finite := m.finite[predicate]; finite != nil {
			if err = m.displayFiniteTableTuples(context, predicate, finite, out); err != nil {
				return err
			}
			continue
		}
		table := m.relations[predicate]
		if table == nil {
			name, getErr := manager.GetASTName(predicate)
			if getErr != nil {
				return getErr
			}
			nameText, getErr := manager.Symbols.Display(name)
			if getErr != nil {
				return getErr
			}
			if _, getErr = fmt.Fprintf(out, "Tuples in %s: \n", nameText); getErr != nil {
				return getErr
			}
			continue
		}
		sorts, _, getErr := m.PredicateSignature(predicate)
		if getErr != nil {
			return getErr
		}
		copy, getErr := table.Clone()
		if getErr != nil {
			return getErr
		}
		relation, getErr := AdoptTableRelation(m, sorts, copy)
		if getErr == nil {
			getErr = relation.DisplayTuples(context, predicate, out)
		}
		getErr = errors.Join(getErr, relation.Close(), copy.Close())
		if getErr != nil {
			return getErr
		}
	}
	return nil
}
